package relay

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRelay(t *testing.T) {
	cfg := Config{
		Sources: []FeedSource{
			{URL: "https://example.com/stream1.m3u8", Label: "Source 1"},
			{URL: "https://example.com/stream2.m3u8", Label: "Source 2"},
		},
		OutputBase: "/tmp/relay-test",
		OutputPath: "test/stream",
		ListenAddr: ":9090",
	}

	r := NewRelay(cfg)

	if len(r.sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(r.sources))
	}
	if r.outputPath != "test/stream" {
		t.Errorf("expected outputPath 'test/stream', got '%s'", r.outputPath)
	}
	if r.maxRestarts != 3 {
		t.Errorf("expected default maxRestarts 3, got %d", r.maxRestarts)
	}
	if r.healthInterval != 5*time.Second {
		t.Errorf("expected default healthInterval 5s, got %v", r.healthInterval)
	}
}

func TestNewRelay_Defaults(t *testing.T) {
	r := NewRelay(Config{
		Sources:    []FeedSource{{URL: "https://example.com/s.m3u8", Label: "S"}},
		OutputBase: "/tmp/relay-test",
	})

	if r.outputPath != "relay/nascar" {
		t.Errorf("expected default outputPath 'relay/nascar', got '%s'", r.outputPath)
	}
	if r.listenAddr != ":8080" {
		t.Errorf("expected default listenAddr ':8080', got '%s'", r.listenAddr)
	}
	if r.hlsSegmentTime != 4 {
		t.Errorf("expected default hlsSegmentTime 4, got %d", r.hlsSegmentTime)
	}
	if r.hlsListSize != 30 {
		t.Errorf("expected default hlsListSize 30, got %d", r.hlsListSize)
	}
}

func TestNewRelay_CustomBuffer(t *testing.T) {
	r := NewRelay(Config{
		Sources:        []FeedSource{{URL: "https://example.com/s.m3u8", Label: "S"}},
		OutputBase:     "/tmp",
		HLSSegmentTime: 6,
		HLSListSize:    60,
	})

	if r.hlsSegmentTime != 6 {
		t.Errorf("expected hlsSegmentTime 6, got %d", r.hlsSegmentTime)
	}
	if r.hlsListSize != 60 {
		t.Errorf("expected hlsListSize 60, got %d", r.hlsListSize)
	}
}

func TestRelay_OutputURL(t *testing.T) {
	r := NewRelay(Config{
		OutputBase: "/tmp",
		OutputPath: "relay/nascar",
	})

	expected := "/hls/relay/nascar/stream.m3u8"
	if r.OutputURL() != expected {
		t.Errorf("expected OutputURL '%s', got '%s'", expected, r.OutputURL())
	}
}

func TestRelay_GetStatus_NotRunning(t *testing.T) {
	r := NewRelay(Config{
		Sources:    []FeedSource{{URL: "https://example.com/s.m3u8", Label: "Source 1"}},
		OutputBase: "/tmp",
	})

	status := r.GetStatus()

	if status.Running {
		t.Error("expected relay to not be running")
	}
	if status.ActiveSource != nil {
		t.Error("expected no active source")
	}
}

func TestRelay_CheckSourceLive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:3.0,\nseg.ts\n"))
	}))
	defer ts.Close()

	r := NewRelay(Config{OutputBase: "/tmp"})
	if !r.checkSourceLive(ts.URL) {
		t.Error("expected live check to pass for valid m3u8")
	}
}

func TestRelay_CheckSourceLive_Down(t *testing.T) {
	r := NewRelay(Config{OutputBase: "/tmp"})
	if r.checkSourceLive("http://localhost:99999/nonexistent.m3u8") {
		t.Error("expected live check to fail for non-existent server")
	}
}

func TestRelay_CheckSourceLive_BadResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	r := NewRelay(Config{OutputBase: "/tmp"})
	if r.checkSourceLive(ts.URL) {
		t.Error("expected live check to fail for 404 response")
	}
}

func TestRelay_IsOutputFresh(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := "test/stream"
	outDir := filepath.Join(tmpDir, outPath)
	os.MkdirAll(outDir, 0755)

	r := NewRelay(Config{OutputBase: tmpDir, OutputPath: outPath})

	if r.isOutputFresh() {
		t.Error("expected output not fresh when file doesn't exist")
	}

	os.WriteFile(filepath.Join(outDir, "stream.m3u8"), []byte("#EXTM3U\n"), 0644)
	if !r.isOutputFresh() {
		t.Error("expected output to be fresh immediately after writing")
	}

	oldTime := time.Now().Add(-30 * time.Second)
	os.Chtimes(filepath.Join(outDir, "stream.m3u8"), oldTime, oldTime)
	if r.isOutputFresh() {
		t.Error("expected output not fresh when file is 30 seconds old")
	}
}

func TestRelay_CleanupDir(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"seg_00001.ts", "seg_00002.ts", "stream.m3u8"} {
		os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644)
	}

	cleanupDir(tmpDir)

	entries, _ := os.ReadDir(tmpDir)
	if len(entries) != 0 {
		t.Errorf("expected 0 files after cleanup, got %d", len(entries))
	}
}

func TestRelay_FindWorkingSource(t *testing.T) {
	tsDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer tsDown.Close()

	tsUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:3.0,\nseg.ts\n"))
	}))
	defer tsUp.Close()

	r := NewRelay(Config{
		Sources: []FeedSource{
			{URL: tsDown.URL + "/down.m3u8", Label: "Down"},
			{URL: tsUp.URL + "/up.m3u8", Label: "Up"},
		},
		OutputBase: t.TempDir(),
	})

	err := r.findWorkingSource()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.activeIdx != 1 {
		t.Errorf("expected activeIdx 1 (Up), got %d", r.activeIdx)
	}
}

func TestRelay_DoubleStart(t *testing.T) {
	r := NewRelay(Config{OutputBase: t.TempDir()})
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	err := r.Start()
	if err == nil {
		t.Error("expected error when starting already running relay")
	}
}

func TestRelay_SwitchSource_NotRunning(t *testing.T) {
	r := NewRelay(Config{
		Sources:    []FeedSource{{URL: "http://a.com/s.m3u8", Label: "A"}},
		OutputBase: t.TempDir(),
	})

	err := r.SwitchSource(0)
	if err == nil {
		t.Error("expected error when switching on stopped relay")
	}
}

func TestRelay_SwitchSource_InvalidIndex(t *testing.T) {
	r := NewRelay(Config{
		Sources:    []FeedSource{{URL: "http://a.com/s.m3u8", Label: "A"}},
		OutputBase: t.TempDir(),
	})
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	err := r.SwitchSource(5)
	if err == nil {
		t.Error("expected error for invalid index")
	}
}

func TestRelay_SwitchSource_SameIndex(t *testing.T) {
	r := NewRelay(Config{
		Sources:    []FeedSource{{URL: "http://a.com/s.m3u8", Label: "A"}},
		OutputBase: t.TempDir(),
	})
	r.mu.Lock()
	r.running = true
	r.activeIdx = 0
	r.mu.Unlock()

	err := r.SwitchSource(0)
	if err != nil {
		t.Errorf("expected no error when switching to same source, got %v", err)
	}
}

func TestRelay_CheckAllSourceHealth(t *testing.T) {
	tsUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:3.0,\nseg.ts\n"))
	}))
	defer tsUp.Close()

	r := NewRelay(Config{
		Sources: []FeedSource{
			{URL: tsUp.URL, Label: "Up"},
			{URL: "http://localhost:99999/dead.m3u8", Label: "Down"},
		},
		OutputBase: t.TempDir(),
	})

	r.checkAllSourceHealth()

	if !r.sources[0].Healthy {
		t.Error("expected first source to be healthy")
	}
	if r.sources[1].Healthy {
		t.Error("expected second source to be unhealthy")
	}
}

// --- Bandwidth probing tests ---

func TestRelay_ProbeSourceBandwidth_MultiVariant(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360
360p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=6000000,RESOLUTION=1920x1080
1080p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720
720p.m3u8
`))
	}))
	defer ts.Close()

	r := NewRelay(Config{OutputBase: t.TempDir()})
	bw, res := r.probeSourceBandwidth(ts.URL)

	if bw != 6000000 {
		t.Errorf("expected max bandwidth 6000000, got %d", bw)
	}
	if res != "1920x1080" {
		t.Errorf("expected resolution '1920x1080', got '%s'", res)
	}
}

func TestRelay_ProbeSourceBandwidth_SingleVariant(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:3.0,\nseg.ts\n"))
	}))
	defer ts.Close()

	r := NewRelay(Config{OutputBase: t.TempDir()})
	bw, _ := r.probeSourceBandwidth(ts.URL)

	if bw != 1 {
		t.Errorf("expected baseline 1 for single-variant, got %d", bw)
	}
}

func TestRelay_ProbeSourceBandwidth_Unreachable(t *testing.T) {
	r := NewRelay(Config{OutputBase: t.TempDir()})
	bw, res := r.probeSourceBandwidth("http://localhost:99999/dead.m3u8")

	if bw != 0 {
		t.Errorf("expected bandwidth 0, got %d", bw)
	}
	if res != "" {
		t.Errorf("expected empty resolution, got '%s'", res)
	}
}

func TestRelay_ProbeSources_SortsByBandwidth(t *testing.T) {
	tsLow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=500000,RESOLUTION=640x360\nlow.m3u8\n"))
	}))
	defer tsLow.Close()

	tsMid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1280x720\nmid.m3u8\n"))
	}))
	defer tsMid.Close()

	tsHigh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=8000000,RESOLUTION=1920x1080\nhigh.m3u8\n"))
	}))
	defer tsHigh.Close()

	r := NewRelay(Config{
		Sources: []FeedSource{
			{URL: tsLow.URL, Label: "Low"},
			{URL: tsHigh.URL, Label: "High"},
			{URL: tsMid.URL, Label: "Mid"},
		},
		OutputBase: t.TempDir(),
	})

	r.ProbeSources()

	if r.sources[0].Label != "High" {
		t.Errorf("expected first = 'High', got '%s'", r.sources[0].Label)
	}
	if r.sources[1].Label != "Mid" {
		t.Errorf("expected second = 'Mid', got '%s'", r.sources[1].Label)
	}
	if r.sources[2].Label != "Low" {
		t.Errorf("expected third = 'Low', got '%s'", r.sources[2].Label)
	}
}

func TestRelay_ProbeSources_UnreachableSinksToBottom(t *testing.T) {
	tsUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720\nlive.m3u8\n"))
	}))
	defer tsUp.Close()

	r := NewRelay(Config{
		Sources: []FeedSource{
			{URL: "http://localhost:99999/dead.m3u8", Label: "Dead"},
			{URL: tsUp.URL, Label: "Alive"},
		},
		OutputBase: t.TempDir(),
	})

	r.ProbeSources()

	if r.sources[0].Label != "Alive" {
		t.Errorf("expected first = 'Alive', got '%s'", r.sources[0].Label)
	}
	if r.sources[1].Label != "Dead" {
		t.Errorf("expected last = 'Dead', got '%s'", r.sources[1].Label)
	}
}
