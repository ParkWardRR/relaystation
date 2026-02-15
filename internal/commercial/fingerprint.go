package commercial

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// AudioFingerprint represents extracted audio features from a stream segment.
// When DD12-INT detects silence (= commercial on other streams), we capture
// fingerprints from those other streams. Over time, these labeled samples
// let us detect commercials on ANY stream without needing DD12-INT.
type AudioFingerprint struct {
	Timestamp    time.Time `json:"timestamp"`
	StreamLabel  string    `json:"stream_label"` // which stream this was captured from
	StreamURL    string    `json:"stream_url,omitempty"`
	EnergyRMS    []float64 `json:"energy_rms"`    // RMS energy per 1s window (linear)
	Loudness     []float64 `json:"loudness"`      // per-second momentary loudness (LUFS)
	PeakDB       float64   `json:"peak_db"`       // peak dB
	MeanDB       float64   `json:"mean_db"`       // mean dB
	StddevDB     float64   `json:"stddev_db"`     // stddev of dB levels (variance = transitions)
	DBRange      float64   `json:"db_range"`      // max - min dB (dynamic range)
	IsCommercial bool      `json:"is_commercial"` // label: true = captured during commercial
	Source       string    `json:"source"`        // how it was labeled ("dd12-int-silence", "manual", "predicted")
	Duration     float64   `json:"duration_sec"`  // duration of captured segment
}

// PatternDB stores learned commercial audio patterns for cross-stream detection.
type PatternDB struct {
	mu       sync.RWMutex
	patterns []AudioFingerprint
	dbPath   string // path for persistence
}

// PatternStats summarizes the learning database.
type PatternStats struct {
	TotalSamples      int    `json:"total_samples"`
	CommercialSamples int    `json:"commercial_samples"`
	NormalSamples     int    `json:"normal_samples"`
	StreamsCovered    int    `json:"streams_covered"`
	DBPath            string `json:"db_path,omitempty"`
	Ready             bool   `json:"ready"` // enough samples to predict
}

// NewPatternDB creates or loads a pattern database.
func NewPatternDB(dbPath string) *PatternDB {
	db := &PatternDB{
		patterns: make([]AudioFingerprint, 0, 256),
		dbPath:   dbPath,
	}
	if dbPath != "" {
		db.load()
	}
	return db
}

// Add adds a labeled fingerprint to the database.
func (db *PatternDB) Add(fp AudioFingerprint) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.patterns = append(db.patterns, fp)
	// Cap at 5000 samples
	if len(db.patterns) > 5000 {
		db.patterns = db.patterns[len(db.patterns)-5000:]
	}
}

// Count returns the number of stored patterns.
func (db *PatternDB) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.patterns)
}

// Stats returns summary statistics.
func (db *PatternDB) Stats() PatternStats {
	db.mu.RLock()
	defer db.mu.RUnlock()

	streams := map[string]bool{}
	commercial, normal := 0, 0
	for _, p := range db.patterns {
		streams[p.StreamLabel] = true
		if p.IsCommercial {
			commercial++
		} else {
			normal++
		}
	}
	return PatternStats{
		TotalSamples:      len(db.patterns),
		CommercialSamples: commercial,
		NormalSamples:     normal,
		StreamsCovered:    len(streams),
		DBPath:            db.dbPath,
		Ready:             commercial >= 5 && normal >= 5,
	}
}

// Predict returns a commercial probability (0-1) for an audio fingerprint
// by comparing it against learned commercial patterns using multiple features:
// energy shape (cosine similarity), mean dB distance, and variance distance.
// Returns -1 if not enough training data.
func (db *PatternDB) Predict(fp AudioFingerprint) float64 {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if len(db.patterns) < 10 {
		return -1 // not enough data
	}

	commScore, normScore := 0.0, 0.0
	commCount, normCount := 0, 0

	for _, p := range db.patterns {
		// Compute multi-feature similarity
		sim := 0.0
		features := 0

		// 1. Energy shape similarity (cosine)
		if len(p.EnergyRMS) > 0 && len(fp.EnergyRMS) > 0 {
			sim += cosineSimilarity(fp.EnergyRMS, p.EnergyRMS)
			features++
		}
		// 2. Loudness shape similarity
		if len(p.Loudness) > 0 && len(fp.Loudness) > 0 {
			sim += cosineSimilarity(fp.Loudness, p.Loudness)
			features++
		}
		// 3. Mean dB distance (closer = more similar)
		if p.MeanDB != 0 && fp.MeanDB != 0 {
			dbDist := math.Abs(fp.MeanDB - p.MeanDB)
			sim += math.Max(0, 1.0-dbDist/30.0) // 30dB range → 0-1
			features++
		}
		// 4. Variance similarity (commercials often have different dynamics)
		if p.StddevDB != 0 && fp.StddevDB != 0 {
			varDist := math.Abs(fp.StddevDB - p.StddevDB)
			sim += math.Max(0, 1.0-varDist/15.0)
			features++
		}

		if features == 0 {
			continue
		}
		avgSim := sim / float64(features)

		if p.IsCommercial {
			commScore += avgSim
			commCount++
		} else {
			normScore += avgSim
			normCount++
		}
	}

	if commCount == 0 || normCount == 0 {
		return -1
	}

	avgComm := commScore / float64(commCount)
	avgNorm := normScore / float64(normCount)

	total := avgComm + avgNorm
	if total == 0 {
		return 0
	}
	return avgComm / total
}

// Save persists the database to disk.
func (db *PatternDB) Save() error {
	if db.dbPath == "" {
		return nil
	}
	db.mu.RLock()
	defer db.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(db.dbPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(db.patterns, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.dbPath, data, 0644)
}

func (db *PatternDB) load() {
	data, err := os.ReadFile(db.dbPath)
	if err != nil {
		return // file doesn't exist yet
	}
	var patterns []AudioFingerprint
	if err := json.Unmarshal(data, &patterns); err != nil {
		log.Printf("[PatternDB] Failed to load %s: %v", db.dbPath, err)
		return
	}
	db.patterns = patterns
	log.Printf("[PatternDB] Loaded %d patterns from %s", len(patterns), db.dbPath)
}

// cosineSimilarity computes similarity between two energy vectors (0-1).
func cosineSimilarity(a, b []float64) float64 {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	if minLen == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < minLen; i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dotProduct / denom
}

// --- Learner: captures fingerprints from other streams during commercials ---

// Learner captures audio fingerprints from other streams when DD12-INT
// detects a commercial. These labeled samples train the pattern matcher.
type Learner struct {
	mu       sync.RWMutex
	detector *Detector
	db       *PatternDB
	streams  []LearnStream // other streams to fingerprint
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// LearnStream is a stream to capture fingerprints from.
type LearnStream struct {
	URL   string
	Label string
}

// NewLearner creates a learner bound to a detector and pattern database.
func NewLearner(det *Detector, db *PatternDB, streams []LearnStream) *Learner {
	return &Learner{
		detector: det,
		db:       db,
		streams:  streams,
	}
}

// Start begins the learner. It watches the detector state and captures
// fingerprints when commercials are detected.
func (l *Learner) Start() {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.running = true
	l.ctx, l.cancel = context.WithCancel(context.Background())
	l.mu.Unlock()

	l.wg.Add(1)
	go l.watchAndCapture()

	log.Printf("[Learner] Started — will capture fingerprints from %d streams during commercials", len(l.streams))
}

// Stop shuts down the learner and persists the database.
func (l *Learner) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.running = false
	l.cancel()
	l.mu.Unlock()
	l.wg.Wait()

	if err := l.db.Save(); err != nil {
		log.Printf("[Learner] Error saving pattern DB: %v", err)
	}
	log.Println("[Learner] Stopped, patterns saved")
}

func (l *Learner) watchAndCapture() {
	defer l.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	wasCommercial := false
	normalCapturedAt := time.Time{}

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			isCommercial := l.detector.IsCommercial()

			// Capture commercial fingerprints when commercial starts
			if isCommercial && !wasCommercial {
				log.Println("[Learner] 📸 Commercial detected — capturing fingerprints from other streams")
				for _, s := range l.streams {
					go l.captureFingerprint(s, true)
				}
			}

			// Periodically capture normal broadcast fingerprints for contrast
			if !isCommercial && time.Since(normalCapturedAt) > 2*time.Minute {
				normalCapturedAt = time.Now()
				for _, s := range l.streams {
					go l.captureFingerprint(s, false)
				}
			}

			wasCommercial = isCommercial
		}
	}
}

// captureFingerprint extracts a 10-second audio fingerprint from a stream.
func (l *Learner) captureFingerprint(stream LearnStream, isCommercial bool) {
	ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
	defer cancel()

	fp, err := extractFingerprint(ctx, stream.URL, stream.Label, 10)
	if err != nil {
		log.Printf("[Learner] Failed to fingerprint %s: %v", stream.Label, err)
		return
	}
	fp.IsCommercial = isCommercial
	if isCommercial {
		fp.Source = "dd12-int-silence"
	} else {
		fp.Source = "normal-broadcast"
	}

	l.db.Add(*fp)

	label := "NORMAL"
	if isCommercial {
		label = "COMMERCIAL"
	}
	log.Printf("[Learner] 📝 Captured %s fingerprint from %s (%.1f dB mean, %d energy samples)",
		label, stream.Label, fp.MeanDB, len(fp.EnergyRMS))

	// Auto-save every 10 samples
	if l.db.Count()%10 == 0 {
		if err := l.db.Save(); err != nil {
			log.Printf("[Learner] Save error: %v", err)
		}
	}
}

// extractFingerprint runs FFmpeg to extract audio features from a stream.
// It captures `durationSec` seconds of audio and computes per-second loudness
// using ebur128 (reliable per-second output) plus silencedetect for silence ratio.
func extractFingerprint(ctx context.Context, streamURL, label string, durationSec int) (*AudioFingerprint, error) {
	// ebur128 outputs per-second momentary loudness (M:) reliably
	args := []string{
		"ffmpeg", "-hide_banner", "-loglevel", "info",
		"-reconnect", "1", "-reconnect_streamed", "1",
		"-i", streamURL,
		"-vn",
		"-t", strconv.Itoa(durationSec),
		"-af", "ebur128=peak=true:framelog=verbose",
		"-f", "null", "-",
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("pipe error: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start error: %w", err)
	}

	// ebur128 outputs lines like:
	// [Parsed_ebur128_0 @ 0x...] t: 1.0   TARGET:-23 LUFS   M: -20.3 S: -19.1 ...
	momentaryRe := regexp.MustCompile(`M:\s*([-\d.]+)`)
	peakRe := regexp.MustCompile(`Peak:\s*([-\d.]+)`)

	var loudness []float64
	var energyRMS []float64
	var peaks []float64

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if m := momentaryRe.FindStringSubmatch(line); len(m) > 1 {
			if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > -120 {
				loudness = append(loudness, val)
				// Convert LUFS to linear energy
				linear := math.Pow(10, val/20)
				energyRMS = append(energyRMS, linear)
			}
		}
		if m := peakRe.FindStringSubmatch(line); len(m) > 1 {
			if val, err := strconv.ParseFloat(m[1], 64); err == nil {
				peaks = append(peaks, val)
			}
		}
	}

	cmd.Wait()

	// Compute statistical features
	meanDB, stddevDB, dbRange, peakDB := computeStats(loudness, peaks)

	return &AudioFingerprint{
		Timestamp:   time.Now(),
		StreamLabel: label,
		StreamURL:   streamURL,
		EnergyRMS:   energyRMS,
		Loudness:    loudness,
		PeakDB:      peakDB,
		MeanDB:      meanDB,
		StddevDB:    stddevDB,
		DBRange:     dbRange,
		Duration:    float64(durationSec),
	}, nil
}

// computeStats derives statistical features from loudness data.
func computeStats(loudness, peaks []float64) (meanDB, stddevDB, dbRange, peakDB float64) {
	if len(loudness) == 0 {
		return 0, 0, 0, 0
	}

	// Mean
	var sum float64
	for _, v := range loudness {
		sum += v
	}
	meanDB = sum / float64(len(loudness))

	// Stddev
	var sumSq float64
	for _, v := range loudness {
		d := v - meanDB
		sumSq += d * d
	}
	stddevDB = math.Sqrt(sumSq / float64(len(loudness)))

	// Range
	minDB, maxDB := loudness[0], loudness[0]
	for _, v := range loudness[1:] {
		if v < minDB {
			minDB = v
		}
		if v > maxDB {
			maxDB = v
		}
	}
	dbRange = maxDB - minDB

	// Peak
	peakDB = -999
	for _, v := range peaks {
		if v > peakDB {
			peakDB = v
		}
	}
	if peakDB == -999 {
		peakDB = maxDB
	}

	return
}

// --- StreamMonitor: independent commercial detection on any stream ---

// StreamMonitor continuously monitors a single stream for commercial patterns
// using the learned PatternDB. It does NOT need DD12-INT — it works purely
// from learned patterns.
type StreamMonitor struct {
	mu         sync.RWMutex
	db         *PatternDB
	stream     LearnStream
	running    bool
	commProb   float64 // last prediction (0-1)
	isComm     bool    // thresholded prediction
	lastCheck  time.Time
	lastFP     *AudioFingerprint
	checkCount int
	commCount  int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// MonitorStatus is the JSON status from a StreamMonitor.
type MonitorStatus struct {
	StreamLabel      string  `json:"stream_label"`
	Running          bool    `json:"running"`
	CommercialProb   float64 `json:"commercial_prob"`
	IsCommercial     bool    `json:"is_commercial"`
	LastCheck        string  `json:"last_check,omitempty"`
	TotalChecks      int     `json:"total_checks"`
	TotalCommercials int     `json:"total_commercials"`
}

// NewStreamMonitor creates a monitor that predicts commercials on a stream.
func NewStreamMonitor(db *PatternDB, stream LearnStream) *StreamMonitor {
	return &StreamMonitor{
		db:     db,
		stream: stream,
	}
}

// Start begins monitoring. Polls every 15s with a 5s audio sample.
func (m *StreamMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.mu.Unlock()

	m.wg.Add(1)
	go m.monitorLoop()
	log.Printf("[Monitor:%s] Started independent commercial monitor", m.stream.Label)
}

// Stop halts the monitor.
func (m *StreamMonitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.cancel()
	m.mu.Unlock()
	m.wg.Wait()
}

// GetStatus returns the monitor status.
func (m *StreamMonitor) GetStatus() MonitorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := MonitorStatus{
		StreamLabel:      m.stream.Label,
		Running:          m.running,
		CommercialProb:   m.commProb,
		IsCommercial:     m.isComm,
		TotalChecks:      m.checkCount,
		TotalCommercials: m.commCount,
	}
	if !m.lastCheck.IsZero() {
		s.LastCheck = m.lastCheck.Format(time.RFC3339)
	}
	return s
}

func (m *StreamMonitor) monitorLoop() {
	defer m.wg.Done()

	// Initial delay to let patterns accumulate
	select {
	case <-time.After(30 * time.Second):
	case <-m.ctx.Done():
		return
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if !m.db.Stats().Ready {
				continue // not enough training data yet
			}
			m.checkOnce()
		}
	}
}

func (m *StreamMonitor) checkOnce() {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	fp, err := extractFingerprint(ctx, m.stream.URL, m.stream.Label, 5)
	if err != nil {
		return
	}

	prob := m.db.Predict(*fp)
	if prob < 0 {
		return // not ready
	}

	m.mu.Lock()
	m.commProb = prob
	m.isComm = prob > 0.6 // 60% threshold for commercial
	m.lastCheck = time.Now()
	m.lastFP = fp
	m.checkCount++
	if m.isComm {
		m.commCount++
	}
	m.mu.Unlock()

	if m.isComm {
		log.Printf("[Monitor:%s] 📺 COMMERCIAL predicted (prob=%.1f%%)", m.stream.Label, prob*100)
	}
}
