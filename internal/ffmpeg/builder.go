package ffmpeg

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/ParkWardRR/relaystation/internal/models"
)

// Builder constructs FFmpeg commands for transcoding
type Builder struct {
	profile         *models.Profile
	upstream        string
	outputDir       string
	defaults        models.Defaults
	characteristics *StreamCharacteristics
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

// SetCharacteristics sets the stream characteristics for dynamic option selection
func (b *Builder) SetCharacteristics(chars *StreamCharacteristics) {
	b.characteristics = chars
}

// Build constructs the full FFmpeg command with dynamic options based on stream type
func (b *Builder) Build() []string {
	p := b.profile

	// Base command
	cmd := []string{"ffmpeg", "-hide_banner", "-loglevel", "warning"}

	// Add input options based on stream characteristics
	cmd = append(cmd, b.buildInputOptions()...)

	// Input
	cmd = append(cmd, "-i", b.upstream)

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

// buildInputOptions generates input options based on stream characteristics
func (b *Builder) buildInputOptions() []string {
	var cmd []string

	// If no characteristics available, use safe defaults (no special options)
	if b.characteristics == nil {
		log.Printf("[FFmpeg] No stream characteristics available, using defaults")
		return cmd
	}

	chars := b.characteristics
	log.Printf("[FFmpeg] Stream type: %s, format: %s, multi-variant: %v, variants: %d",
		chars.StreamType, chars.SegmentFormat, chars.IsMultiVariant, chars.VariantCount)

	// For LIVE streams, add reconnection options
	if chars.StreamType == StreamTypeLive {
		cmd = append(cmd,
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
			"-reconnect_on_network_error", "1",
		)
		log.Printf("[FFmpeg] Live stream detected - enabling reconnection")
	}

	// For VOD streams, don't add reconnection options (they cause EOF loops)
	if chars.StreamType == StreamTypeVOD {
		log.Printf("[FFmpeg] VOD stream detected - no reconnection options")
		// No special options needed - FFmpeg handles VOD well by default
	}

	// For fMP4 streams, use slightly larger probe size for better demuxing
	// but don't go overboard - let FFmpeg auto-select streams
	if chars.SegmentFormat == SegmentFormatFMP4 {
		log.Printf("[FFmpeg] fMP4 format detected - using default demuxer (no special options needed)")
		// FFmpeg's HLS demuxer handles fMP4 well without special options
	}

	// For multi-variant playlists with many variants, FFmpeg handles selection automatically
	if chars.IsMultiVariant && chars.VariantCount > 1 {
		log.Printf("[FFmpeg] Multi-variant playlist with %d variants - letting FFmpeg auto-select", chars.VariantCount)
		// FFmpeg selects highest bandwidth by default, which is what we want
	}

	return cmd
}
