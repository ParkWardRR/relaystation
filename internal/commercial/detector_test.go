package commercial

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector(Config{
		StreamURL:   "https://example.com/stream.m3u8",
		StreamLabel: "TEST",
	})
	if d.cfg.SilenceThresholdDB != -30 {
		t.Errorf("expected default threshold -30, got %.0f", d.cfg.SilenceThresholdDB)
	}
	if d.cfg.CommercialSec != 10 {
		t.Errorf("expected default commercial threshold 10s, got %.0f", d.cfg.CommercialSec)
	}
	if d.state != StateNormal {
		t.Errorf("expected initial state Normal, got %s", d.state)
	}
}

func TestDetector_GetStatus_NotRunning(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST"})
	s := d.GetStatus()
	if s.Monitoring {
		t.Error("expected not monitoring")
	}
	if s.State != StateNormal {
		t.Errorf("expected state normal, got %s", s.State)
	}
}

func TestDetector_ParseSilenceOutput_SilenceStart(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST", CommercialSec: 10})

	input := "[silencedetect @ 0x123] silence_start: 45.678\n"
	d.ParseSilenceOutput(strings.NewReader(input))

	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.state != StateSilence {
		t.Errorf("expected state silence, got %s", d.state)
	}
	if d.silenceStart == nil {
		t.Error("expected silenceStart to be set")
	}
}

func TestDetector_ParseSilenceOutput_SilenceEnd(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST"})

	// Simulate silence start then end
	d.onSilenceStart()
	input := "[silencedetect @ 0x123] silence_end: 50.123 | silence_duration: 4.445\n"
	d.ParseSilenceOutput(strings.NewReader(input))

	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.state != StateNormal {
		t.Errorf("expected state normal, got %s", d.state)
	}
	if d.lastSpeech == nil {
		t.Error("expected lastSpeech to be set")
	}
}

func TestDetector_CommercialPromotion(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST", CommercialSec: 0.1})
	d.ctx, d.cancel = makeTestContext()
	d.running = true

	d.wg.Add(1)
	go d.stateChecker()

	d.onSilenceStart()
	time.Sleep(1500 * time.Millisecond) // wait for 1s ticker to check

	d.mu.RLock()
	state := d.state
	breaks := len(d.breaks)
	d.mu.RUnlock()

	if state != StateCommercial {
		t.Errorf("expected state commercial, got %s", state)
	}
	if breaks != 1 {
		t.Errorf("expected 1 commercial break, got %d", breaks)
	}

	d.cancel()
	d.wg.Wait()
}

func TestDetector_CommercialEndUpdatesBreak(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST", CommercialSec: 0.05})

	// Simulate full commercial cycle
	d.onSilenceStart()
	time.Sleep(100 * time.Millisecond)

	// Manually promote to commercial
	d.mu.Lock()
	d.state = StateCommercial
	d.breaks = append(d.breaks, CommercialBreak{Start: *d.silenceStart})
	d.mu.Unlock()

	d.onSilenceEnd(5.0)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.state != StateNormal {
		t.Errorf("expected state normal after commercial end, got %s", d.state)
	}
	if len(d.breaks) != 1 {
		t.Fatalf("expected 1 break, got %d", len(d.breaks))
	}
	if d.breaks[0].End == nil {
		t.Error("expected break End to be set")
	}
}

func TestDetector_SilenceRegex(t *testing.T) {
	tests := []struct {
		line    string
		isStart bool
		isEnd   bool
	}{
		{"[silencedetect @ 0x7f8] silence_start: 123.456", true, false},
		{"[silencedetect @ 0x7f8] silence_end: 135.789 | silence_duration: 12.333", false, true},
		{"[hls @ 0x123] keepalive request failed", false, false},
		{"frame= 1234 fps=30", false, false},
	}

	for _, tt := range tests {
		if got := silenceStartRe.MatchString(tt.line); got != tt.isStart {
			t.Errorf("silenceStartRe on %q: got %v, want %v", tt.line, got, tt.isStart)
		}
		if got := silenceEndRe.MatchString(tt.line); got != tt.isEnd {
			t.Errorf("silenceEndRe on %q: got %v, want %v", tt.line, got, tt.isEnd)
		}
	}
}

func TestDetector_IsCommercial(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST"})

	if d.IsCommercial() {
		t.Error("expected not commercial initially")
	}

	d.mu.Lock()
	d.state = StateCommercial
	d.mu.Unlock()

	if !d.IsCommercial() {
		t.Error("expected commercial after state change")
	}
}

func TestDetector_DoubleStart(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST"})
	d.mu.Lock()
	d.running = true
	d.mu.Unlock()

	err := d.Start()
	if err == nil {
		t.Error("expected error on double start")
	}
}

func TestDetector_ParseMultipleEvents(t *testing.T) {
	d := NewDetector(Config{StreamLabel: "TEST", CommercialSec: 100})

	input := `[silencedetect @ 0x1] silence_start: 10.000
[silencedetect @ 0x1] silence_end: 13.500 | silence_duration: 3.500
[silencedetect @ 0x1] silence_start: 20.000
[silencedetect @ 0x1] silence_end: 25.000 | silence_duration: 5.000
`
	d.ParseSilenceOutput(strings.NewReader(input))

	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.state != StateNormal {
		t.Errorf("expected normal after all events, got %s", d.state)
	}
	if d.lastSpeech == nil {
		t.Error("expected lastSpeech to be set")
	}
}

// --- Fingerprint tests ---

func TestFingerprint_Serialization(t *testing.T) {
	fp := AudioFingerprint{
		Timestamp:    time.Now(),
		StreamLabel:  "ACE-FOX",
		EnergyRMS:    []float64{0.01, 0.02, 0.5, 0.6, 0.01},
		IsCommercial: true,
		Source:       "dd12-int-silence",
	}

	if len(fp.EnergyRMS) != 5 {
		t.Errorf("expected 5 energy samples, got %d", len(fp.EnergyRMS))
	}
	if !fp.IsCommercial {
		t.Error("expected IsCommercial true")
	}
}

func TestPatternDB_AddAndMatch(t *testing.T) {
	db := NewPatternDB("")

	fp := AudioFingerprint{
		Timestamp:    time.Now(),
		StreamLabel:  "ACE-FOX",
		EnergyRMS:    []float64{0.01, 0.01, 0.5, 0.6, 0.01, 0.01},
		IsCommercial: true,
		Source:       "dd12-int-silence",
	}
	db.Add(fp)

	if db.Count() != 1 {
		t.Errorf("expected 1 pattern, got %d", db.Count())
	}

	stats := db.Stats()
	if stats.TotalSamples != 1 {
		t.Errorf("expected 1 total sample, got %d", stats.TotalSamples)
	}
}

func TestEnergyPattern_Similarity(t *testing.T) {
	a := []float64{0.01, 0.01, 0.5, 0.6, 0.01}
	b := []float64{0.01, 0.02, 0.48, 0.59, 0.01}
	c := []float64{0.5, 0.5, 0.5, 0.5, 0.5}

	simAB := cosineSimilarity(a, b)
	simAC := cosineSimilarity(a, c)

	if simAB < 0.95 {
		t.Errorf("expected high similarity between similar patterns, got %.3f", simAB)
	}
	if simAC > simAB {
		t.Errorf("expected dissimilar pattern to have lower score (%.3f > %.3f)", simAC, simAB)
	}
}

// helper
func makeTestContext() (ctx, cancel) {
	return context.WithCancel(context.Background())
}

type ctx = context.Context
type cancel = context.CancelFunc
