package scanner

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// MediaSource represents a discovered media URL from a web page
type MediaSource struct {
	URL      string `json:"url"`
	Type     string `json:"type"`     // "m3u8", "mpd", "embed"
	Label    string `json:"label"`    // descriptive label (e.g., "FOX", "TSN")
	SourcePage string `json:"source_page"` // page it was found on
}

// ScanResult contains all discovered media sources from a page
type ScanResult struct {
	PageURL string        `json:"page_url"`
	Sources []MediaSource `json:"sources"`
	Error   string        `json:"error,omitempty"`
}

// Scanner extracts media URLs from web pages
type Scanner struct {
	client *http.Client
	// Patterns for extracting stream URLs
	m3u8Pattern  *regexp.Regexp
	mpdPattern   *regexp.Regexp
	iframeSrcPattern *regexp.Regexp
	buttonPattern    *regexp.Regexp
}

// NewScanner creates a new media URL scanner
func NewScanner() *Scanner {
	return &Scanner{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		m3u8Pattern:  regexp.MustCompile(`https?://[^\s"'<>]+?\.m3u8[^\s"'<>]*`),
		mpdPattern:   regexp.MustCompile(`https?://[^\s"'<>]+?\.mpd[^\s"'<>]*`),
		iframeSrcPattern: regexp.MustCompile(`(?i)<iframe[^>]+src=["']([^"']+)["']`),
		buttonPattern:    regexp.MustCompile(`(?i)onclick=["'][^"']*(?:src\s*=\s*'([^']+)'|src\s*=\s*"([^"]+)").*?>(.*?)</button>`),
	}
}

// ScanPage fetches a web page and extracts all media source URLs
func (s *Scanner) ScanPage(pageURL string) *ScanResult {
	result := &ScanResult{
		PageURL: pageURL,
		Sources: make([]MediaSource, 0),
	}

	body, err := s.fetchPage(pageURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	seen := make(map[string]bool)

	// Extract m3u8 URLs
	for _, match := range s.m3u8Pattern.FindAllString(body, -1) {
		clean := cleanURL(match)
		if !seen[clean] {
			seen[clean] = true
			result.Sources = append(result.Sources, MediaSource{
				URL:        clean,
				Type:       "m3u8",
				Label:      guessLabel(clean, body),
				SourcePage: pageURL,
			})
		}
	}

	// Extract MPD URLs
	for _, match := range s.mpdPattern.FindAllString(body, -1) {
		clean := cleanURL(match)
		if !seen[clean] {
			seen[clean] = true
			result.Sources = append(result.Sources, MediaSource{
				URL:        clean,
				Type:       "mpd",
				Label:      guessLabel(clean, body),
				SourcePage: pageURL,
			})
		}
	}

	// Extract iframe sources (these might contain embedded players)
	for _, match := range s.iframeSrcPattern.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			src := match[1]
			if !seen[src] && (strings.Contains(src, "m3u8") || strings.Contains(src, "stream") || strings.Contains(src, "embed")) {
				seen[src] = true
				result.Sources = append(result.Sources, MediaSource{
					URL:        src,
					Type:       "embed",
					Label:      "embedded",
					SourcePage: pageURL,
				})
			}
		}
	}

	log.Printf("[Scanner] Found %d sources on %s", len(result.Sources), pageURL)
	return result
}

// ScanPages scans multiple pages and returns all results
func (s *Scanner) ScanPages(pageURLs []string) []*ScanResult {
	results := make([]*ScanResult, 0, len(pageURLs))
	for _, url := range pageURLs {
		results = append(results, s.ScanPage(url))
	}
	return results
}

// ExtractM3U8URLs scans pages and returns only direct m3u8 URLs suitable for relay
func (s *Scanner) ExtractM3U8URLs(pageURLs []string) []MediaSource {
	var sources []MediaSource
	seen := make(map[string]bool)

	for _, result := range s.ScanPages(pageURLs) {
		for _, src := range result.Sources {
			if src.Type == "m3u8" && !seen[src.URL] {
				seen[src.URL] = true
				sources = append(sources, src)
			}
		}
	}

	return sources
}

func (s *Scanner) fetchPage(pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, pageURL)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

// cleanURL removes trailing characters that aren't part of the URL
func cleanURL(u string) string {
	// Remove trailing quotes, angle brackets, semicolons, etc
	u = strings.TrimRight(u, `"'<>;,)]}`)
	// Remove trailing backslash
	u = strings.TrimRight(u, `\`)
	return u
}

// guessLabel attempts to find a descriptive label near the URL in the HTML
func guessLabel(url, html string) string {
	// Look for button text before the URL
	idx := strings.Index(html, url)
	if idx == -1 {
		return ""
	}

	// Search backwards for a button/label
	before := html[max(0, idx-200):idx]

	// Look for button text like >FOX</button>
	btnRe := regexp.MustCompile(`>([A-Z0-9]{2,10})<`)
	matches := btnRe.FindAllStringSubmatch(before, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1][1]
	}

	return ""
}
