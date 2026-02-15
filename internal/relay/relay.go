// Package relay provides a multi-source HLS passthrough relay with automatic
// failover and bandwidth-prioritized source selection.
//
// The relay accepts multiple upstream m3u8 sources, probes them for bandwidth,
// sorts by highest bitrate first, and relays the active source via FFmpeg
// passthrough (no transcoding). If the active source dies, it automatically
// fails over to the next available source.
//
// A built-in web dashboard at /relay provides real-time monitoring and
// one-click source switching.
package relay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FeedSource represents a single upstream HLS feed with probed metadata.
// Sources are probed on startup for bandwidth/resolution and periodically
// for health (liveness). The relay sorts sources by MaxBandwidth descending
// so the highest quality feed is always preferred.
type FeedSource struct {
	URL           string `json:"url"`                      // m3u8 URL of the upstream source
	Label         string `json:"label"`                    // human-readable label (e.g., "DD12-US")
	Active        bool   `json:"active"`                   // true if this source is currently being relayed
	MaxBandwidth  int    `json:"max_bandwidth,omitempty"`  // max BANDWIDTH from EXT-X-STREAM-INF (bps)
	MaxResolution string `json:"max_resolution,omitempty"` // max RESOLUTION (e.g., "1920x1080")
	Probed        bool   `json:"probed"`                   // whether bandwidth has been probed
	Healthy       bool   `json:"healthy"`                  // last liveness check result
}

// Status represents a point-in-time snapshot of the relay, returned by
// the GET /api/relay/status endpoint and consumed by the dashboard.
type Status struct {
	Running       bool         `json:"running"`        // whether the relay is active
	ActiveSource  *FeedSource  `json:"active_source"`  // the source currently being relayed
	ActiveIdx     int          `json:"active_idx"`     // index of the active source in AllSources
	AllSources    []FeedSource `json:"all_sources"`    // all configured sources with health/bandwidth info
	RestartCount  int          `json:"restart_count"`  // total FFmpeg restart count
	FailoverCount int          `json:"failover_count"` // number of source-to-source failovers
	Uptime        string       `json:"uptime"`         // human-readable uptime (e.g., "2m30s")
	OutputURL     string       `json:"output_url"`     // full URL to the output HLS playlist
}

// Relay manages a single active FFmpeg passthrough relay with fast source switching.
// It wraps a single FFmpeg process that copies (passthrough) the active upstream
// m3u8 feed to a local HLS output directory. The SwitchSource method enables
// instant source changes by killing the old FFmpeg process and starting a new one.
type Relay struct {
	mu             sync.RWMutex
	sources        []FeedSource
	activeIdx      int
	outputBase     string
	outputPath     string
	listenAddr     string
	running        bool
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	ctx            context.Context
	masterCtx      context.Context
	masterCancel   context.CancelFunc
	restartCount   int
	failoverCount  int
	startTime      time.Time
	healthInterval time.Duration
	wg             sync.WaitGroup
	maxRestarts    int
	sourceRestarts int
	hlsSegmentTime int
	hlsListSize    int
}

// Config holds relay configuration. All fields have sensible defaults;
// only Sources and OutputBase are required.
type Config struct {
	Sources        []FeedSource
	OutputBase     string
	OutputPath     string
	ListenAddr     string
	HealthInterval time.Duration
	MaxRestarts    int
	HLSSegmentTime int
	HLSListSize    int
}

// NewRelay creates a new relay
func NewRelay(cfg Config) *Relay {
	if cfg.HealthInterval == 0 {
		cfg.HealthInterval = 5 * time.Second
	}
	if cfg.MaxRestarts == 0 {
		cfg.MaxRestarts = 3
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = "relay/nascar"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.HLSSegmentTime == 0 {
		cfg.HLSSegmentTime = 4
	}
	if cfg.HLSListSize == 0 {
		cfg.HLSListSize = 30
	}

	return &Relay{
		sources:        cfg.Sources,
		activeIdx:      0,
		outputBase:     cfg.OutputBase,
		outputPath:     cfg.OutputPath,
		listenAddr:     cfg.ListenAddr,
		healthInterval: cfg.HealthInterval,
		maxRestarts:    cfg.MaxRestarts,
		hlsSegmentTime: cfg.HLSSegmentTime,
		hlsListSize:    cfg.HLSListSize,
	}
}

// Start begins the relay
func (r *Relay) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("relay is already running")
	}
	r.running = true
	r.startTime = time.Now()
	masterCtx, masterCancel := context.WithCancel(context.Background())
	r.masterCtx = masterCtx
	r.masterCancel = masterCancel
	r.mu.Unlock()

	log.Println("[Relay] Starting relay...")
	log.Printf("[Relay] %d sources configured", len(r.sources))
	log.Printf("[Relay] Buffer: %d segments × %ds = ~%ds",
		r.hlsListSize, r.hlsSegmentTime, r.hlsListSize*r.hlsSegmentTime)

	// Probe and sort by bandwidth
	r.ProbeSources()

	// Check health of all sources
	r.checkAllSourceHealth()

	for i, s := range r.sources {
		health := "✗"
		if s.Healthy {
			health = "✓"
		}
		if s.MaxBandwidth > 0 {
			log.Printf("[Relay]   [%d] %s %s — %s (%d bps)", i, health, s.Label, s.MaxResolution, s.MaxBandwidth)
		} else {
			log.Printf("[Relay]   [%d] %s %s [bitrate unknown]", i, health, s.Label)
		}
	}

	// Find first working source
	if err := r.findWorkingSource(); err != nil {
		r.mu.Lock()
		r.running = false
		r.masterCancel()
		r.mu.Unlock()
		return fmt.Errorf("no working sources: %w", err)
	}

	// Start FFmpeg
	if err := r.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Health check loop
	r.wg.Add(1)
	go r.healthCheckLoop()

	return nil
}

// Stop shuts down the relay
func (r *Relay) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	if r.cancel != nil {
		r.cancel()
	}
	r.masterCancel()
	r.mu.Unlock()

	r.wg.Wait()
	log.Println("[Relay] Stopped")
}

// SwitchSource switches to a specific source by index — fast, kills old FFmpeg immediately
func (r *Relay) SwitchSource(idx int) error {
	r.mu.RLock()
	if !r.running {
		r.mu.RUnlock()
		return fmt.Errorf("relay is not running")
	}
	if idx < 0 || idx >= len(r.sources) {
		r.mu.RUnlock()
		return fmt.Errorf("invalid source index %d (have %d sources)", idx, len(r.sources))
	}
	if idx == r.activeIdx {
		r.mu.RUnlock()
		return nil // already on this source
	}
	r.mu.RUnlock()

	src := r.sources[idx]
	log.Printf("[Relay] ⚡ SWITCH to [%d] %s", idx, src.Label)

	// Kill current FFmpeg immediately
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.activeIdx = idx
	r.sourceRestarts = 0
	r.mu.Unlock()

	// Small delay for process cleanup
	time.Sleep(200 * time.Millisecond)

	// Clean old segments for seamless transition
	outputDir := filepath.Join(r.outputBase, r.outputPath)
	cleanupDir(outputDir)

	// Start new FFmpeg
	return r.startFFmpeg()
}

// GetStatus returns current relay status with health info
func (r *Relay) GetStatus() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build sources list with active flag
	srcs := make([]FeedSource, len(r.sources))
	copy(srcs, r.sources)

	status := Status{
		Running:       r.running,
		ActiveIdx:     r.activeIdx,
		AllSources:    srcs,
		RestartCount:  r.restartCount,
		FailoverCount: r.failoverCount,
	}

	if r.running {
		status.Uptime = time.Since(r.startTime).Round(time.Second).String()
		if r.activeIdx < len(r.sources) {
			src := r.sources[r.activeIdx]
			src.Active = true
			status.ActiveSource = &src
			srcs[r.activeIdx].Active = true
		}
		status.OutputURL = fmt.Sprintf("http://YOUR_IP%s/hls/%s/stream.m3u8", r.listenAddr, r.outputPath)
	}

	return status
}

// OutputURL returns the HLS output URL path
func (r *Relay) OutputURL() string {
	return fmt.Sprintf("/hls/%s/stream.m3u8", r.outputPath)
}

// findWorkingSource finds the first source that responds, starting from activeIdx
func (r *Relay) findWorkingSource() error {
	startIdx := r.activeIdx
	for i := 0; i < len(r.sources); i++ {
		idx := (startIdx + i) % len(r.sources)
		src := r.sources[idx]

		log.Printf("[Relay] Checking source [%d] %s", idx, src.Label)
		if r.checkSourceLive(src.URL) {
			r.mu.Lock()
			r.activeIdx = idx
			r.sourceRestarts = 0
			r.sources[idx].Healthy = true
			r.mu.Unlock()
			log.Printf("[Relay] ✓ Source [%d] %s is live", idx, src.Label)
			return nil
		}
		r.mu.Lock()
		r.sources[idx].Healthy = false
		r.mu.Unlock()
		log.Printf("[Relay] ✗ Source [%d] %s is not responding", idx, src.Label)
	}
	return fmt.Errorf("all %d sources are down", len(r.sources))
}

// failover moves to the next source
func (r *Relay) failover() error {
	r.mu.Lock()
	r.failoverCount++
	oldIdx := r.activeIdx
	r.activeIdx = (r.activeIdx + 1) % len(r.sources)
	r.sourceRestarts = 0
	r.mu.Unlock()

	log.Printf("[Relay] ⚡ FAILOVER from [%d] to [%d]", oldIdx, r.activeIdx)

	if r.cancel != nil {
		r.cancel()
	}

	// Clean old segments
	outputDir := filepath.Join(r.outputBase, r.outputPath)
	cleanupDir(outputDir)

	if err := r.findWorkingSource(); err != nil {
		log.Printf("[Relay] ⚠ All sources down, retrying in 10s...")
		r.mu.Lock()
		r.activeIdx = 0
		r.mu.Unlock()
		time.Sleep(10 * time.Second)
		return r.failover()
	}

	return r.startFFmpeg()
}

// startFFmpeg starts FFmpeg passthrough for the active source
func (r *Relay) startFFmpeg() error {
	r.mu.RLock()
	src := r.sources[r.activeIdx]
	r.mu.RUnlock()

	outputDir := filepath.Join(r.outputBase, r.outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	outputFile := filepath.Join(outputDir, "stream.m3u8")

	ctx, cancel := context.WithCancel(r.masterCtx)
	r.mu.Lock()
	r.ctx = ctx
	r.cancel = cancel
	r.mu.Unlock()

	args := []string{
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		// Large probe buffer
		"-probesize", "10000000",
		"-analyzeduration", "5000000",
		// Reconnection
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "10",
		"-reconnect_on_network_error", "1",
		// Timeout
		"-rw_timeout", "15000000",
		// Input
		"-i", src.URL,
		// Passthrough
		"-c", "copy",
		// HLS output with large buffer
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", r.hlsSegmentTime),
		"-hls_list_size", fmt.Sprintf("%d", r.hlsListSize),
		"-hls_flags", "delete_segments+append_list+split_by_time",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", filepath.Join(outputDir, "seg_%05d.ts"),
		"-hls_allow_cache", "1",
		"-start_number", "0",
		outputFile,
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	r.mu.Lock()
	r.cmd = cmd
	r.mu.Unlock()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}

	log.Printf("[Relay] ▶ Relaying [%d] %s (PID: %d)", r.activeIdx, src.Label, cmd.Process.Pid)

	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			log.Printf("[Relay:FFmpeg] %s", s.Text())
		}
	}()

	r.wg.Add(1)
	go r.monitorProcess(ctx)

	return nil
}

// monitorProcess watches FFmpeg and handles crashes
func (r *Relay) monitorProcess(ctx context.Context) {
	defer r.wg.Done()

	r.mu.RLock()
	cmd := r.cmd
	r.mu.RUnlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	select {
	case <-ctx.Done():
		return
	default:
	}

	r.mu.RLock()
	running := r.running
	r.mu.RUnlock()
	if !running {
		return
	}

	r.mu.Lock()
	r.restartCount++
	r.sourceRestarts++
	sr := r.sourceRestarts
	mr := r.maxRestarts
	r.mu.Unlock()

	if err != nil {
		log.Printf("[Relay] ⚠ FFmpeg exited: %v (restart %d/%d)", err, sr, mr)
	} else {
		log.Printf("[Relay] ⚠ FFmpeg exited unexpectedly (restart %d/%d)", sr, mr)
	}

	if sr >= mr {
		log.Printf("[Relay] Max restarts hit, failing over...")
		if err := r.failover(); err != nil {
			log.Printf("[Relay] Failover error: %v", err)
		}
		return
	}

	time.Sleep(2 * time.Second)
	if err := r.startFFmpeg(); err != nil {
		log.Printf("[Relay] Restart failed, failing over: %v", err)
		if err := r.failover(); err != nil {
			log.Printf("[Relay] Failover error: %v", err)
		}
	}
}

// healthCheckLoop monitors output freshness and source health
func (r *Relay) healthCheckLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.healthInterval)
	defer ticker.Stop()

	time.Sleep(15 * time.Second)

	consecutiveFailures := 0
	healthCheckCounter := 0

	for {
		r.mu.RLock()
		running := r.running
		r.mu.RUnlock()
		if !running {
			return
		}

		<-ticker.C

		// Check output freshness
		if r.isOutputFresh() {
			consecutiveFailures = 0
		} else {
			consecutiveFailures++
			log.Printf("[Relay] ⚠ Output stale (%d consecutive)", consecutiveFailures)
			if consecutiveFailures >= 3 {
				consecutiveFailures = 0
				go func() {
					if err := r.failover(); err != nil {
						log.Printf("[Relay] Failover error: %v", err)
					}
				}()
			}
		}

		// Periodically check all source health (every 30s)
		healthCheckCounter++
		if healthCheckCounter >= 6 {
			healthCheckCounter = 0
			go r.checkAllSourceHealth()
		}
	}
}

// checkAllSourceHealth probes all sources for liveness
func (r *Relay) checkAllSourceHealth() {
	var wg sync.WaitGroup
	for i := range r.sources {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			live := r.checkSourceLive(r.sources[idx].URL)
			r.mu.Lock()
			r.sources[idx].Healthy = live
			r.mu.Unlock()
		}(i)
	}
	wg.Wait()
}

// isOutputFresh checks if the HLS playlist was recently modified
func (r *Relay) isOutputFresh() bool {
	playlistPath := filepath.Join(r.outputBase, r.outputPath, "stream.m3u8")
	info, err := os.Stat(playlistPath)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < 15*time.Second
}

// checkSourceLive checks if a URL responds with valid m3u8
func (r *Relay) checkSourceLive(url string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return false
	}

	buf := make([]byte, 1024)
	n, err := io.ReadAtLeast(resp.Body, buf, 7)
	if err != nil && n < 7 {
		return false
	}
	content := string(buf[:n])
	return len(content) > 0 && (content[0] == '#' || resp.StatusCode == 200)
}

// ProbeSources probes all sources for bandwidth and sorts highest first
func (r *Relay) ProbeSources() {
	log.Println("[Relay] Probing sources for bandwidth...")

	type probeResult struct {
		idx        int
		bandwidth  int
		resolution string
	}

	results := make(chan probeResult, len(r.sources))
	var wg sync.WaitGroup

	for i, src := range r.sources {
		wg.Add(1)
		go func(idx int, url, label string) {
			defer wg.Done()
			bw, res := r.probeSourceBandwidth(url)
			results <- probeResult{idx: idx, bandwidth: bw, resolution: res}
		}(i, src.URL, src.Label)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		r.sources[res.idx].MaxBandwidth = res.bandwidth
		r.sources[res.idx].MaxResolution = res.resolution
		r.sources[res.idx].Probed = true
	}

	sort.SliceStable(r.sources, func(i, j int) bool {
		return r.sources[i].MaxBandwidth > r.sources[j].MaxBandwidth
	})

	log.Println("[Relay] Sources sorted by bandwidth (highest first)")
}

// probeSourceBandwidth extracts max BANDWIDTH from m3u8 manifest
func (r *Relay) probeSourceBandwidth(m3u8URL string) (int, string) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(m3u8URL)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return 0, ""
	}

	bandwidthRe := regexp.MustCompile(`BANDWIDTH=(\d+)`)
	resolutionRe := regexp.MustCompile(`RESOLUTION=(\d+x\d+)`)

	maxBW := 0
	maxRes := ""

	s := bufio.NewScanner(resp.Body)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			if m := bandwidthRe.FindStringSubmatch(line); len(m) > 1 {
				bw, _ := strconv.Atoi(m[1])
				if bw > maxBW {
					maxBW = bw
					if rm := resolutionRe.FindStringSubmatch(line); len(rm) > 1 {
						maxRes = rm[1]
					}
				}
			}
		}

		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") && maxBW == 0 {
			maxBW = 1
		}
	}

	return maxBW, maxRes
}

// cleanupDir removes all files from a directory
func cleanupDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
