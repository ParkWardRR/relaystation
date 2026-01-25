package ffmpeg

import (
	"bufio"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ParkWardRR/relaystation/internal/models"
)

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
