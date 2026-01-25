package ffmpeg

import (
	"bufio"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/ParkWardRR/relaystation/internal/models"
)

// Process manages a single FFmpeg transcoding process
type Process struct {
	profile      *models.Profile
	upstream     string
	defaults     models.Defaults
	outputBase   string
	cmd          *exec.Cmd
	running      bool
	restartCount int
	maxRestarts  int
	restartDelay time.Duration
	mu           sync.Mutex
	cancel       context.CancelFunc
}

// NewProcess creates a new FFmpeg process wrapper
func NewProcess(profile *models.Profile, upstream, outputBase string, defaults models.Defaults) *Process {
	return &Process{
		profile:      profile,
		upstream:     upstream,
		defaults:     defaults,
		outputBase:   outputBase,
		maxRestarts:  10,
		restartDelay: 5 * time.Second,
	}
}

// Start begins the FFmpeg process
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	// Create output directory
	outputDir := filepath.Join(p.outputBase, p.profile.Path)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Probe the upstream to detect stream characteristics
	log.Printf("[FFmpeg] Probing upstream: %s", p.upstream)
	chars, err := ProbeStreamCharacteristics(p.upstream)
	if err != nil {
		log.Printf("[FFmpeg] Failed to probe stream characteristics: %v (using defaults)", err)
		chars = nil
	}

	// Build command with characteristics
	builder := NewBuilder(p.profile, p.upstream, outputDir, p.defaults)
	builder.SetCharacteristics(chars)
	args := builder.Build()

	// Create context with cancel
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	// Create command
	p.cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	// Capture stderr for logging
	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Start process
	if err := p.cmd.Start(); err != nil {
		return err
	}

	p.running = true
	log.Printf("[FFmpeg] Started process for profile %s (PID: %d)", p.profile.ID, p.cmd.Process.Pid)

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[FFmpeg:%s] %s", p.profile.ID, scanner.Text())
		}
	}()

	// Monitor process in background
	go p.monitor(ctx)

	return nil
}

func (p *Process) monitor(ctx context.Context) {
	// Wait for process to exit
	err := p.cmd.Wait()

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	// Check if context was cancelled (intentional stop)
	select {
	case <-ctx.Done():
		log.Printf("[FFmpeg] Process %s stopped (context cancelled)", p.profile.ID)
		return
	default:
	}

	// Process crashed, attempt restart
	if err != nil {
		log.Printf("[FFmpeg] Process %s exited with error: %v", p.profile.ID, err)

		p.mu.Lock()
		p.restartCount++
		count := p.restartCount
		p.mu.Unlock()

		if count <= p.maxRestarts {
			log.Printf("[FFmpeg] Restarting process %s (attempt %d/%d)", p.profile.ID, count, p.maxRestarts)
			time.Sleep(p.restartDelay)

			// Restart with new context
			newCtx := context.Background()
			if err := p.Start(newCtx); err != nil {
				log.Printf("[FFmpeg] Failed to restart %s: %v", p.profile.ID, err)
			}
		} else {
			log.Printf("[FFmpeg] Max restarts reached for %s, giving up", p.profile.ID)
		}
	}
}

// Stop terminates the FFmpeg process
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running || p.cancel == nil {
		return nil
	}

	log.Printf("[FFmpeg] Stopping process for profile %s", p.profile.ID)
	p.cancel()
	p.running = false

	return nil
}

// IsRunning returns whether the process is currently running
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// RestartCount returns the number of restarts
func (p *Process) RestartCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.restartCount
}

// Profile returns the profile this process is encoding
func (p *Process) Profile() *models.Profile {
	return p.profile
}

// IsLive checks if the HLS output is available
func (p *Process) IsLive() bool {
	playlistPath := filepath.Join(p.outputBase, p.profile.Path, "stream.m3u8")
	info, err := os.Stat(playlistPath)
	if err != nil {
		return false
	}

	// Check if playlist was modified recently (within 30 seconds)
	return time.Since(info.ModTime()) < 30*time.Second
}
