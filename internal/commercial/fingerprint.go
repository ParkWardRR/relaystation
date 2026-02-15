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
	EnergyRMS    []float64 `json:"energy_rms"`    // RMS energy per 1s window
	PeakDB       float64   `json:"peak_db"`       // peak dB
	MeanDB       float64   `json:"mean_db"`       // mean dB
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
// by comparing it against learned commercial patterns using cosine similarity.
// Returns -1 if not enough training data.
func (db *PatternDB) Predict(fp AudioFingerprint) float64 {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if len(db.patterns) < 10 {
		return -1 // not enough data
	}

	commercialSim, normalSim := 0.0, 0.0
	commercialCount, normalCount := 0, 0

	for _, p := range db.patterns {
		if len(p.EnergyRMS) == 0 {
			continue
		}
		sim := cosineSimilarity(fp.EnergyRMS, p.EnergyRMS)
		if p.IsCommercial {
			commercialSim += sim
			commercialCount++
		} else {
			normalSim += sim
			normalCount++
		}
	}

	if commercialCount == 0 || normalCount == 0 {
		return -1
	}

	avgCommercial := commercialSim / float64(commercialCount)
	avgNormal := normalSim / float64(normalCount)

	// Higher similarity to commercial patterns = higher probability
	total := avgCommercial + avgNormal
	if total == 0 {
		return 0
	}
	return avgCommercial / total
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
// It captures `durationSec` seconds of audio and computes per-second RMS energy.
func extractFingerprint(ctx context.Context, streamURL, label string, durationSec int) (*AudioFingerprint, error) {
	// Use FFmpeg astats filter for per-frame audio statistics
	args := []string{
		"ffmpeg", "-hide_banner", "-loglevel", "info",
		"-reconnect", "1", "-reconnect_streamed", "1",
		"-i", streamURL,
		"-vn",
		"-t", strconv.Itoa(durationSec),
		"-af", "astats=metadata=1:reset=1:length=1",
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

	// Parse astats output for RMS levels
	rmsRe := regexp.MustCompile(`RMS level dB:\s*([-\d.]+)`)
	peakRe := regexp.MustCompile(`Peak level dB:\s*([-\d.]+)`)

	var energyRMS []float64
	var peakDB, sumDB float64
	count := 0

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if m := rmsRe.FindStringSubmatch(line); len(m) > 1 {
			if val, err := strconv.ParseFloat(m[1], 64); err == nil {
				// Convert dB to linear energy for the fingerprint
				linear := math.Pow(10, val/20)
				energyRMS = append(energyRMS, linear)
				sumDB += val
				count++
			}
		}
		if m := peakRe.FindStringSubmatch(line); len(m) > 1 {
			if val, err := strconv.ParseFloat(m[1], 64); err == nil {
				if val > peakDB || peakDB == 0 {
					peakDB = val
				}
			}
		}
	}

	cmd.Wait()

	meanDB := 0.0
	if count > 0 {
		meanDB = sumDB / float64(count)
	}

	return &AudioFingerprint{
		Timestamp:   time.Now(),
		StreamLabel: label,
		StreamURL:   streamURL,
		EnergyRMS:   energyRMS,
		PeakDB:      peakDB,
		MeanDB:      meanDB,
		Duration:    float64(durationSec),
	}, nil
}
