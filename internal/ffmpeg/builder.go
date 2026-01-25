package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ParkWardRR/relaystation/internal/models"
)

// Builder constructs FFmpeg commands for transcoding
type Builder struct {
	profile   *models.Profile
	upstream  string
	outputDir string
	defaults  models.Defaults
}

// NewBuilder creates a new FFmpeg command builder
func NewBuilder(profile *models.Profile, upstream, outputDir string, defaults models.Defaults) *Builder {
	return &Builder{
		profile:   profile,
		upstream:  upstream,
		outputDir: outputDir,
		defaults:  defaults,
	}
}

// Build constructs the full FFmpeg command
func (b *Builder) Build() []string {
	p := b.profile

	// Base command - keep it simple for best HLS compatibility
	// Let FFmpeg auto-select the best video/audio streams from master playlist
	cmd := []string{
		"ffmpeg", "-hide_banner", "-loglevel", "warning",
		"-i", b.upstream,
	}

	// If passthrough, just copy streams
	if p.Passthrough {
		cmd = append(cmd, "-c", "copy")
	} else {
		// Video encoding
		cmd = append(cmd, b.buildVideoCodec()...)
		// Audio encoding
		cmd = append(cmd, b.buildAudioCodec()...)
	}

	// HLS output options
	cmd = append(cmd, b.buildHLSOutput()...)

	return cmd
}

func (b *Builder) buildVideoCodec() []string {
	p := b.profile
	var cmd []string

	switch strings.ToLower(p.Codec) {
	case "h264":
		cmd = append(cmd, "-c:v", "libx264")
		if p.ProfileName != "" {
			cmd = append(cmd, "-profile:v", p.ProfileName)
		}
		if p.Level != "" {
			cmd = append(cmd, "-level:v", p.Level)
		}
		cmd = append(cmd, "-preset", b.defaults.Preset)
		cmd = append(cmd, "-tune", "zerolatency")

	case "h265", "hevc":
		cmd = append(cmd, "-c:v", "libx265")
		cmd = append(cmd, "-tag:v", "hvc1") // Apple compatibility
		if p.ProfileName != "" {
			cmd = append(cmd, "-profile:v", p.ProfileName)
		}
		cmd = append(cmd, "-preset", b.defaults.Preset)
		cmd = append(cmd, "-x265-params", "log-level=error")

	default:
		cmd = append(cmd, "-c:v", "copy")
	}

	// Resolution scaling
	if p.Resolution != "" {
		parts := strings.Split(p.Resolution, "x")
		if len(parts) == 2 {
			// Scale with padding to maintain aspect ratio
			filter := fmt.Sprintf("scale=%s:%s:force_original_aspect_ratio=decrease,pad=%s:%s:(ow-iw)/2:(oh-ih)/2",
				parts[0], parts[1], parts[0], parts[1])
			cmd = append(cmd, "-vf", filter)
		}
	}

	// Bitrate
	if p.Bitrate != "" {
		cmd = append(cmd, "-b:v", p.Bitrate)
	}
	if p.MaxRate != "" {
		cmd = append(cmd, "-maxrate", p.MaxRate)
		// Buffer size = 2x maxrate
		bufsize := strings.TrimSuffix(p.MaxRate, "k")
		cmd = append(cmd, "-bufsize", bufsize+"k")
	}

	// Frame rate
	if p.FPS > 0 {
		cmd = append(cmd, "-r", fmt.Sprintf("%d", p.FPS))
		// Keyframe interval = 2x FPS
		gop := p.FPS * 2
		cmd = append(cmd, "-g", fmt.Sprintf("%d", gop))
		cmd = append(cmd, "-keyint_min", fmt.Sprintf("%d", p.FPS))
	}

	// Pixel format for compatibility
	cmd = append(cmd, "-pix_fmt", "yuv420p")

	return cmd
}

func (b *Builder) buildAudioCodec() []string {
	p := b.profile
	cmd := []string{
		"-c:a", "aac",
		"-ac", "2", // Stereo
	}

	if p.AudioBitrate != "" {
		cmd = append(cmd, "-b:a", p.AudioBitrate)
	} else {
		cmd = append(cmd, "-b:a", "128k")
	}

	if p.AudioSample != "" {
		cmd = append(cmd, "-ar", p.AudioSample)
	} else {
		cmd = append(cmd, "-ar", "44100")
	}

	return cmd
}

func (b *Builder) buildHLSOutput() []string {
	outputPath := filepath.Join(b.outputDir, "stream.m3u8")

	cmd := []string{
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", b.defaults.SegmentTime),
		"-hls_list_size", fmt.Sprintf("%d", b.defaults.PlaylistSize),
		"-hls_flags", "delete_segments+append_list+split_by_time",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", filepath.Join(b.outputDir, "segment_%03d.ts"),
		outputPath,
	}

	return cmd
}
