// Package commercial provides real-time commercial break detection by
// monitoring audio silence patterns in live HLS streams.
//
// It uses FFmpeg with the silencedetect audio filter to identify periods
// of silence. When announcers go silent for a configurable duration
// (default: 10 seconds), it signals a likely commercial break on other
// streams. Optionally integrates with a CoreML sound classifier
// (Apple Neural Engine / GPU accelerated) for enhanced speech detection.
package commercial

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// State represents the current detection state.
type State string

const (
	StateNormal     State = "normal"     // audio is active (speech/music)
	StateSilence    State = "silence"    // audio is silent but below commercial threshold
	StateCommercial State = "commercial" // extended silence — commercial break likely
)

// CommercialBreak records a detected commercial break period.
type CommercialBreak struct {
	Start    time.Time  `json:"start"`
	End      *time.Time `json:"end,omitempty"`
	Duration string     `json:"duration,omitempty"`
}

// ClassifierOutput holds a single result from the CoreML sound classifier.
type ClassifierOutput struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Time       float64 `json:"time"`
}

// Status is the JSON-serializable status of the commercial detector.
type Status struct {
	Monitoring       bool              `json:"monitoring"`
	StreamLabel      string            `json:"stream_label"`
	State            State             `json:"state"`
	SilenceDurationS float64           `json:"silence_duration_sec"`
	LastSpeechAt     string            `json:"last_speech_at,omitempty"`
	CommercialBreaks []CommercialBreak `json:"commercial_breaks"`
	TotalCommercials int               `json:"total_commercials"`
	TotalCommTime    string            `json:"total_commercial_time"`
	Classifier       *ClassifierInfo   `json:"classifier,omitempty"`
	Uptime           string            `json:"uptime"`
}

// ClassifierInfo shows CoreML classifier state.
type ClassifierInfo struct {
	Enabled    bool    `json:"enabled"`
	Available  bool    `json:"available"`
	TopLabel   string  `json:"top_label,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// Config holds detector configuration.
type Config struct {
	StreamURL          string  // m3u8 URL to monitor
	StreamLabel        string  // human-readable label (e.g. "DD12-INT")
	SilenceThresholdDB float64 // dB noise floor (default: -30)
	MinSilenceSec      float64 // minimum silence to detect (default: 1)
	CommercialSec      float64 // silence duration for commercial flag (default: 10)
	ClassifierPath     string  // path to CoreML classifier binary (optional)
}

// Detector monitors an HLS stream for silence / commercial breaks.
type Detector struct {
	mu  sync.RWMutex
	cfg Config

	// runtime
	running      bool
	state        State
	silenceStart *time.Time
	lastSpeech   *time.Time
	startTime    time.Time

	// history (keep last 50)
	breaks    []CommercialBreak
	totalComm time.Duration

	// classifier
	classifierOut *ClassifierOutput
	classifierOK  bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewDetector creates a new commercial detector.
func NewDetector(cfg Config) *Detector {
	if cfg.SilenceThresholdDB == 0 {
		cfg.SilenceThresholdDB = -30
	}
	if cfg.MinSilenceSec == 0 {
		cfg.MinSilenceSec = 1
	}
	if cfg.CommercialSec == 0 {
		cfg.CommercialSec = 10
	}
	return &Detector{
		cfg:    cfg,
		state:  StateNormal,
		breaks: make([]CommercialBreak, 0),
	}
}

// Start begins monitoring. Non-blocking.
func (d *Detector) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("detector already running")
	}
	d.running = true
	d.startTime = time.Now()
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.mu.Unlock()

	log.Printf("[Commercial] Starting silence detector on %s", d.cfg.StreamLabel)
	log.Printf("[Commercial] Threshold: %.0fdB, commercial after %.0fs silence",
		d.cfg.SilenceThresholdDB, d.cfg.CommercialSec)

	d.wg.Add(1)
	go d.runSilenceDetect()

	// Optional CoreML classifier
	if d.cfg.ClassifierPath != "" {
		if _, err := os.Stat(d.cfg.ClassifierPath); err == nil {
			d.mu.Lock()
			d.classifierOK = true
			d.mu.Unlock()
			d.wg.Add(1)
			go d.runClassifier()
		} else {
			log.Printf("[Commercial] CoreML classifier not found at %s, skipping", d.cfg.ClassifierPath)
		}
	}

	// State checker goroutine
	d.wg.Add(1)
	go d.stateChecker()

	return nil
}

// Stop shuts down the detector.
func (d *Detector) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	d.cancel()
	d.mu.Unlock()
	d.wg.Wait()
	log.Println("[Commercial] Detector stopped")
}

// GetStatus returns a snapshot of the detector state.
func (d *Detector) GetStatus() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	s := Status{
		Monitoring:       d.running,
		StreamLabel:      d.cfg.StreamLabel,
		State:            d.state,
		CommercialBreaks: d.breaks,
		TotalCommercials: len(d.breaks),
		TotalCommTime:    d.totalComm.Round(time.Second).String(),
	}

	if d.running {
		s.Uptime = time.Since(d.startTime).Round(time.Second).String()
	}
	if d.lastSpeech != nil {
		s.LastSpeechAt = d.lastSpeech.Format(time.RFC3339)
	}
	if d.silenceStart != nil {
		s.SilenceDurationS = time.Since(*d.silenceStart).Seconds()
	}
	if d.classifierOK {
		ci := &ClassifierInfo{Enabled: true, Available: true}
		if d.classifierOut != nil {
			ci.TopLabel = d.classifierOut.Label
			ci.Confidence = d.classifierOut.Confidence
		}
		s.Classifier = ci
	}
	return s
}

// IsCommercial returns true if a commercial break is currently detected.
func (d *Detector) IsCommercial() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state == StateCommercial
}

// --- FFmpeg silence detection ---

var (
	silenceStartRe = regexp.MustCompile(`silence_start:\s*([\d.]+)`)
	silenceEndRe   = regexp.MustCompile(`silence_end:\s*([\d.]+)\s*\|\s*silence_duration:\s*([\d.]+)`)
)

func (d *Detector) runSilenceDetect() {
	defer d.wg.Done()

	for {
		if d.ctx.Err() != nil {
			return
		}
		d.runSilenceDetectOnce()

		d.mu.RLock()
		running := d.running
		d.mu.RUnlock()
		if !running {
			return
		}
		log.Println("[Commercial] FFmpeg exited, restarting in 3s...")
		select {
		case <-time.After(3 * time.Second):
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *Detector) runSilenceDetectOnce() {
	args := []string{
		"ffmpeg", "-hide_banner", "-loglevel", "info",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "10",
		"-i", d.cfg.StreamURL,
		"-vn", // discard video — audio only
		"-af", fmt.Sprintf("silencedetect=noise=%.0fdB:d=%.0f",
			d.cfg.SilenceThresholdDB, d.cfg.MinSilenceSec),
		"-f", "null", "-",
	}

	cmd := exec.CommandContext(d.ctx, args[0], args[1:]...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("[Commercial] stderr pipe error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[Commercial] FFmpeg start error: %v", err)
		return
	}
	log.Printf("[Commercial] ▶ FFmpeg silencedetect started (PID: %d)", cmd.Process.Pid)

	d.parseSilenceOutput(stderr)
	cmd.Wait()
}

// ParseSilenceOutput parses FFmpeg silencedetect stderr output.
// Exported for testing.
func (d *Detector) ParseSilenceOutput(r io.Reader) {
	d.parseSilenceOutput(r)
}

func (d *Detector) parseSilenceOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if m := silenceStartRe.FindStringSubmatch(line); len(m) > 1 {
			d.onSilenceStart()
		} else if m := silenceEndRe.FindStringSubmatch(line); len(m) > 2 {
			dur, _ := strconv.ParseFloat(m[2], 64)
			d.onSilenceEnd(dur)
		}
	}
}

func (d *Detector) onSilenceStart() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	d.silenceStart = &now
	if d.state == StateNormal {
		d.state = StateSilence
		log.Println("[Commercial] 🔇 Silence detected")
	}
}

func (d *Detector) onSilenceEnd(durationSec float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	d.lastSpeech = &now
	wasCommercial := d.state == StateCommercial

	if wasCommercial && d.silenceStart != nil {
		// Close the commercial break
		dur := time.Since(*d.silenceStart)
		if len(d.breaks) > 0 {
			last := &d.breaks[len(d.breaks)-1]
			if last.End == nil {
				last.End = &now
				last.Duration = dur.Round(time.Second).String()
				d.totalComm += dur
			}
		}
		log.Printf("[Commercial] 🔊 Commercial break ended (%.1fs)", durationSec)
	} else if d.state == StateSilence {
		log.Printf("[Commercial] 🔊 Silence ended (%.1fs — below commercial threshold)", durationSec)
	}

	d.state = StateNormal
	d.silenceStart = nil
}

// stateChecker promotes silence → commercial when threshold exceeded
func (d *Detector) stateChecker() {
	defer d.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.mu.Lock()
			if d.state == StateSilence && d.silenceStart != nil {
				elapsed := time.Since(*d.silenceStart).Seconds()
				if elapsed >= d.cfg.CommercialSec {
					d.state = StateCommercial
					d.breaks = append(d.breaks, CommercialBreak{
						Start: *d.silenceStart,
					})
					// cap history
					if len(d.breaks) > 50 {
						d.breaks = d.breaks[len(d.breaks)-50:]
					}
					log.Printf("[Commercial] 📺 COMMERCIAL BREAK detected (%.0fs silence)", elapsed)
				}
			}
			d.mu.Unlock()
		}
	}
}

// --- Optional CoreML classifier ---

func (d *Detector) runClassifier() {
	defer d.wg.Done()

	log.Printf("[Commercial] Starting CoreML sound classifier (%s)", d.cfg.ClassifierPath)

	// FFmpeg extracts 16kHz mono f32le PCM, pipes to classifier
	ffArgs := []string{
		"ffmpeg", "-hide_banner", "-loglevel", "error",
		"-reconnect", "1", "-reconnect_streamed", "1",
		"-reconnect_delay_max", "10",
		"-i", d.cfg.StreamURL,
		"-vn",
		"-f", "f32le", "-acodec", "pcm_f32le",
		"-ac", "1", "-ar", "16000",
		"pipe:1",
	}

	ffCmd := exec.CommandContext(d.ctx, ffArgs[0], ffArgs[1:]...)
	ffStdout, err := ffCmd.StdoutPipe()
	if err != nil {
		log.Printf("[Commercial] classifier ffmpeg pipe error: %v", err)
		return
	}
	ffCmd.Stderr = nil

	clCmd := exec.CommandContext(d.ctx, d.cfg.ClassifierPath)
	clCmd.Stdin = ffStdout
	clStdout, err := clCmd.StdoutPipe()
	if err != nil {
		log.Printf("[Commercial] classifier stdout pipe error: %v", err)
		return
	}

	if err := ffCmd.Start(); err != nil {
		log.Printf("[Commercial] classifier ffmpeg start error: %v", err)
		return
	}
	if err := clCmd.Start(); err != nil {
		log.Printf("[Commercial] classifier start error: %v", err)
		ffCmd.Process.Kill()
		return
	}

	log.Printf("[Commercial] ▶ CoreML classifier running (Neural Engine / GPU accelerated)")

	scanner := bufio.NewScanner(clStdout)
	for scanner.Scan() {
		var result ClassifierOutput
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			continue
		}
		d.mu.Lock()
		d.classifierOut = &result
		d.mu.Unlock()
	}

	ffCmd.Wait()
	clCmd.Wait()
}
