package ffmpeg

import (
	"bufio"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ParkWardRR/relaystation/internal/models"
)

// StreamType indicates whether the stream is live or VOD
type StreamType string

const (
	StreamTypeLive    StreamType = "live"
	StreamTypeVOD     StreamType = "vod"
	StreamTypeUnknown StreamType = "unknown"
)

// SegmentFormat indicates the container format of segments
type SegmentFormat string

const (
	SegmentFormatMPEGTS  SegmentFormat = "mpegts"
	SegmentFormatFMP4    SegmentFormat = "fmp4"
	SegmentFormatUnknown SegmentFormat = "unknown"
)

// StreamCharacteristics contains detected properties of an HLS stream
type StreamCharacteristics struct {
	StreamType      StreamType
	SegmentFormat   SegmentFormat
	IsMultiVariant  bool
	HasSubtitles    bool
	HasAudio        bool
	TargetDuration  int
	MaxBandwidth    int
	MaxResolution   string
	VariantCount    int
}

// ProbeStreamCharacteristics analyzes an HLS URL to determine optimal FFmpeg options
func ProbeStreamCharacteristics(hlsURL string) (*StreamCharacteristics, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get(hlsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	chars := &StreamCharacteristics{
		StreamType:    StreamTypeUnknown,
		SegmentFormat: SegmentFormatUnknown,
		HasAudio:      true, // Assume audio by default
	}

	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}

	// Analyze the manifest
	for i, line := range lines {
		// Detect VOD by presence of ENDLIST
		if line == "#EXT-X-ENDLIST" {
			chars.StreamType = StreamTypeVOD
		}

		// Detect multi-variant playlist
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			chars.IsMultiVariant = true
			chars.VariantCount++

			// Parse bandwidth
			if matches := regexp.MustCompile(`BANDWIDTH=(\d+)`).FindStringSubmatch(line); len(matches) > 1 {
				bw, _ := strconv.Atoi(matches[1])
				if bw > chars.MaxBandwidth {
					chars.MaxBandwidth = bw
				}
			}

			// Parse resolution
			if matches := regexp.MustCompile(`RESOLUTION=(\d+x\d+)`).FindStringSubmatch(line); len(matches) > 1 {
				chars.MaxResolution = matches[1]
			}
		}

		// Detect subtitles
		if strings.Contains(line, "TYPE=SUBTITLES") || strings.Contains(line, "SUBTITLES=") {
			chars.HasSubtitles = true
		}

		// Detect audio tracks
		if strings.Contains(line, "TYPE=AUDIO") {
			chars.HasAudio = true
		}

		// Detect fMP4 by EXT-X-MAP (initialization segment)
		if strings.HasPrefix(line, "#EXT-X-MAP:") {
			chars.SegmentFormat = SegmentFormatFMP4
		}

		// Detect target duration
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			if val := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"); val != "" {
				chars.TargetDuration, _ = strconv.Atoi(val)
			}
		}

		// Detect segment format from segment extensions
		if !strings.HasPrefix(line, "#") && line != "" {
			if strings.HasSuffix(line, ".ts") {
				if chars.SegmentFormat == SegmentFormatUnknown {
					chars.SegmentFormat = SegmentFormatMPEGTS
				}
			} else if strings.HasSuffix(line, ".m4s") || strings.HasSuffix(line, ".m4v") || strings.HasSuffix(line, ".m4a") || strings.HasSuffix(line, ".mp4") {
				chars.SegmentFormat = SegmentFormatFMP4
			}
		}

		// For multi-variant, we need to probe a child playlist to get more info
		if chars.IsMultiVariant && i == len(lines)-1 && chars.StreamType == StreamTypeUnknown {
			// Try to probe the first variant
			for _, l := range lines {
				if !strings.HasPrefix(l, "#") && l != "" {
					childURL := resolveURL(hlsURL, l)
					childChars, err := ProbeStreamCharacteristics(childURL)
					if err == nil {
						// Inherit stream type and segment format from child
						if childChars.StreamType != StreamTypeUnknown {
							chars.StreamType = childChars.StreamType
						}
						if childChars.SegmentFormat != SegmentFormatUnknown {
							chars.SegmentFormat = childChars.SegmentFormat
						}
						if childChars.TargetDuration > 0 {
							chars.TargetDuration = childChars.TargetDuration
						}
					}
					break
				}
			}
		}
	}

	// Default to live if no ENDLIST found and we have segments
	if chars.StreamType == StreamTypeUnknown && chars.SegmentFormat != SegmentFormatUnknown {
		chars.StreamType = StreamTypeLive
	}

	return chars, nil
}

// resolveURL resolves a relative URL against a base URL
func resolveURL(baseURL, relativeURL string) string {
	if strings.HasPrefix(relativeURL, "http://") || strings.HasPrefix(relativeURL, "https://") {
		return relativeURL
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return relativeURL
	}

	ref, err := url.Parse(relativeURL)
	if err != nil {
		return relativeURL
	}

	return base.ResolveReference(ref).String()
}

// ProbeSource fetches and parses an HLS master playlist to get quality variants
func ProbeSource(url string) (*models.SourceInfo, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	info := &models.SourceInfo{
		Variants: make([]models.SourceVariant, 0),
	}

	// Parse M3U8 master playlist
	bandwidthRe := regexp.MustCompile(`BANDWIDTH=(\d+)`)
	resolutionRe := regexp.MustCompile(`RESOLUTION=(\d+x\d+)`)
	codecsRe := regexp.MustCompile(`CODECS="([^"]+)"`)

	var currentVariant *models.SourceVariant
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			currentVariant = &models.SourceVariant{}

			// Extract bandwidth
			if matches := bandwidthRe.FindStringSubmatch(line); len(matches) > 1 {
				currentVariant.Bandwidth, _ = strconv.Atoi(matches[1])
			}

			// Extract resolution
			if matches := resolutionRe.FindStringSubmatch(line); len(matches) > 1 {
				currentVariant.Resolution = matches[1]
			}

			// Extract codecs
			if matches := codecsRe.FindStringSubmatch(line); len(matches) > 1 {
				currentVariant.Codecs = matches[1]
			}
		} else if currentVariant != nil && !strings.HasPrefix(line, "#") && line != "" {
			currentVariant.URI = line
			info.Variants = append(info.Variants, *currentVariant)
			currentVariant = nil
		}
	}

	// Determine max quality
	if len(info.Variants) > 0 {
		maxBandwidth := 0
		maxRes := ""
		for _, v := range info.Variants {
			if v.Bandwidth > maxBandwidth {
				maxBandwidth = v.Bandwidth
				maxRes = v.Resolution
			}
		}
		info.MaxQuality = maxRes
	}

	return info, nil
}

// CheckUpstreamLive verifies if the upstream URL is accessible
func CheckUpstreamLive(url string) bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
