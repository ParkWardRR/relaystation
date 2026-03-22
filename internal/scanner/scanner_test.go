package scanner

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScanner_ScanPage_ExtractsM3U8(t *testing.T) {
	// Create a test server with HTML containing m3u8 URLs
	html := `<html>
	<head><title>Test</title></head>
	<body>
		<button onclick="document.getElementById('iframe').src = '/iframe1?s=https://example.com/stream1/index.m3u8'">FOX</button>
		<button onclick="document.getElementById('iframe').src = '/iframe1?s=https://example.com/stream2/index.m3u8'">TSN</button>
		<iframe id='iframe' src="/iframe1?s=https://example.com/default/index.m3u8"></iframe>
	</body>
	</html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	s := NewScanner()
	result := s.ScanPage(ts.URL)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	if len(result.Sources) == 0 {
		t.Fatal("expected at least one source, got 0")
	}

	// Should find the m3u8 URLs
	foundM3U8 := false
	for _, src := range result.Sources {
		if src.Type == "m3u8" {
			foundM3U8 = true
			t.Logf("Found m3u8: %s (label: %s)", src.URL, src.Label)
		}
	}

	if !foundM3U8 {
		t.Error("expected to find m3u8 sources")
	}
}

func TestScanner_ScanPage_ExtractsMPD(t *testing.T) {
	html := `<html><body>
		<button onclick="document.getElementById('iframe').src = '/shaka?mpd=https://example.com/stream.mpd&keyId=abc&key=def'">ZIGGO</button>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	s := NewScanner()
	result := s.ScanPage(ts.URL)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	foundMPD := false
	for _, src := range result.Sources {
		if src.Type == "mpd" {
			foundMPD = true
			t.Logf("Found MPD: %s", src.URL)
		}
	}

	if !foundMPD {
		t.Error("expected to find MPD source")
	}
}

func TestScanner_ScanPage_DeduplicatesURLs(t *testing.T) {
	html := `<html><body>
		<button onclick="src='https://example.com/stream1/index.m3u8'">CH1</button>
		<iframe src="/iframe1?s=https://example.com/stream1/index.m3u8"></iframe>
		<script>var url = "https://example.com/stream1/index.m3u8";</script>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	s := NewScanner()
	result := s.ScanPage(ts.URL)

	m3u8Count := 0
	for _, src := range result.Sources {
		if src.Type == "m3u8" {
			m3u8Count++
		}
	}

	if m3u8Count != 1 {
		t.Errorf("expected 1 unique m3u8 URL, got %d", m3u8Count)
	}
}

func TestScanner_ScanPage_HandlesErrors(t *testing.T) {
	s := NewScanner()
	result := s.ScanPage("http://localhost:99999/nonexistent")

	if result.Error == "" {
		t.Error("expected error for non-existent server")
	}
	if len(result.Sources) != 0 {
		t.Error("expected no sources on error")
	}
}

func TestScanner_ExtractM3U8URLs(t *testing.T) {
	html1 := `<html><body>
		<script>var src = "https://cdn1.example.com/stream/index.m3u8";</script>
	</body></html>`

	html2 := `<html><body>
		<script>var src = "https://cdn2.example.com/live/master.m3u8";</script>
	</body></html>`

	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html1))
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html2))
	}))
	defer ts2.Close()

	s := NewScanner()
	sources := s.ExtractM3U8URLs([]string{ts1.URL, ts2.URL})

	if len(sources) != 2 {
		t.Errorf("expected 2 m3u8 URLs, got %d", len(sources))
	}

	for _, src := range sources {
		if src.Type != "m3u8" {
			t.Errorf("expected type m3u8, got %s", src.Type)
		}
		t.Logf("Found: %s", src.URL)
	}
}

func TestScanner_ScanPage_IMSALikePage(t *testing.T) {
	// Simulate a page similar to the real IMSA streaming pages
	html := `<!--<script language="JavaScript">
window.location="/";
</script>-->
<html lang="en">
<head>
    <title>Live</title>
    <script type='text/javascript' src='https://cdn.jsdelivr.net/clappr/latest/clappr.min.js'></script>
</head>
<body>
<div class='links centerize'>
<button class="btn" onclick="document.getElementById('iframe').src = '/iframe1?s=https://bozztv.com/dvrfl05/gin-fox5/index.m3u8'">FOX</button>
<button class="btn" onclick="document.getElementById('iframe').src = '/iframe1?s=https://stream.decentdoubts.net/809/index.m3u8'">TSN</button>
<button class="btn" onclick="document.getElementById('iframe').src = '/shaka?mpd=https://example.com/stream.mpd&keyId=abc&key=def'">ZIGGO</button>
</div>
<div class="wrapper">
  <div id="stream" class="stream bigstream">
    <iframe id='iframe' src="/iframe1?s=https://bozztv.com/dvrfl05/gin-fox5/index.m3u8" allowFullScreen="true"></iframe>
  </div>
</div>
</body>
</html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	s := NewScanner()
	result := s.ScanPage(ts.URL)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	t.Logf("Found %d sources:", len(result.Sources))
	for _, src := range result.Sources {
		t.Logf("  [%s] %s (label: %s)", src.Type, src.URL, src.Label)
	}

	// Should find at least the m3u8 URLs
	m3u8Count := 0
	mpdCount := 0
	for _, src := range result.Sources {
		switch src.Type {
		case "m3u8":
			m3u8Count++
		case "mpd":
			mpdCount++
		}
	}

	if m3u8Count < 2 {
		t.Errorf("expected at least 2 m3u8 URLs, got %d", m3u8Count)
	}
	if mpdCount < 1 {
		t.Errorf("expected at least 1 MPD URL, got %d", mpdCount)
	}
}

func TestCleanURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`https://example.com/stream.m3u8"`,
			"https://example.com/stream.m3u8",
		},
		{
			`https://example.com/stream.m3u8'`,
			"https://example.com/stream.m3u8",
		},
		{
			`https://example.com/stream.m3u8\`,
			"https://example.com/stream.m3u8",
		},
		{
			"https://example.com/stream.m3u8",
			"https://example.com/stream.m3u8",
		},
	}

	for _, tc := range tests {
		result := cleanURL(tc.input)
		if result != tc.expected {
			t.Errorf("cleanURL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
