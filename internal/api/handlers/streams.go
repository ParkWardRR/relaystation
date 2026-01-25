package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/ffmpeg"
	"github.com/ParkWardRR/relaystation/internal/models"
	"github.com/ParkWardRR/relaystation/internal/stream"
)

// StreamHandler handles stream-related API requests
type StreamHandler struct {
	config  *config.Manager
	manager *stream.Manager
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(cfg *config.Manager, mgr *stream.Manager) *StreamHandler {
	return &StreamHandler{config: cfg, manager: mgr}
}

// List returns all streams
func (h *StreamHandler) List(c *fiber.Ctx) error {
	streams := h.config.GetStreams()
	return c.JSON(streams)
}

// Get returns a specific stream
func (h *StreamHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	stream := h.config.GetStream(id)

	if stream == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Stream not found",
		})
	}

	return c.JSON(stream)
}

// Create adds a new stream
func (h *StreamHandler) Create(c *fiber.Ctx) error {
	var req struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Upstream string `json:"upstream"`
		Preset   string `json:"preset,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.ID == "" || req.Upstream == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ID and upstream are required",
		})
	}

	// Check if stream already exists
	if h.config.GetStream(req.ID) != nil {
		return c.Status(409).JSON(fiber.Map{
			"error": "Stream already exists",
		})
	}

	// Create stream
	stream := models.Stream{
		ID:       req.ID,
		Name:     req.Name,
		Upstream: req.Upstream,
		Enabled:  true,
		Profiles: make(map[string]*models.Profile),
	}

	// Apply default preset if specified
	if req.Preset != "" {
		if preset, ok := config.BuiltinPresets[req.Preset]; ok {
			profile := &models.Profile{
				ID:           req.ID + "_" + preset.ID,
				Enabled:      true,
				Passthrough:  preset.ID == "passthrough",
				Path:         "/" + preset.ID + "/" + req.ID + "/",
				Codec:        preset.Codec,
				ProfileName:  preset.ProfileName,
				Level:        preset.Level,
				Resolution:   preset.Resolution,
				Bitrate:      preset.Bitrate,
				MaxRate:      preset.MaxRate,
				FPS:          preset.FPS,
				AudioBitrate: preset.AudioBitrate,
				AudioSample:  preset.AudioSample,
			}
			stream.Profiles[preset.ID] = profile
		}
	}

	if err := h.config.AddStream(stream); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to add stream",
		})
	}

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	h.manager.Sync()

	return c.Status(201).JSON(stream)
}

// Update modifies an existing stream
func (h *StreamHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	if h.config.GetStream(id) == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Stream not found",
		})
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.config.UpdateStream(id, updates); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to update stream",
		})
	}

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	h.manager.Sync()

	return c.JSON(h.config.GetStream(id))
}

// Delete removes a stream
func (h *StreamHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	if h.config.GetStream(id) == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Stream not found",
		})
	}

	if err := h.config.DeleteStream(id); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to delete stream",
		})
	}

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	h.manager.Sync()

	return c.JSON(fiber.Map{
		"message": "Stream deleted",
	})
}

// ApplyPreset applies a transcoding preset to a stream
func (h *StreamHandler) ApplyPreset(c *fiber.Ctx) error {
	id := c.Params("id")

	if h.config.GetStream(id) == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Stream not found",
		})
	}

	var preset models.Preset
	if err := c.BodyParser(&preset); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.config.ApplyPresetToStream(id, preset); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to apply preset",
		})
	}

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	h.manager.Sync()

	return c.JSON(fiber.Map{
		"message": "Preset applied",
	})
}

// GetSourceInfo probes the upstream source for quality variants
func (h *StreamHandler) GetSourceInfo(c *fiber.Ctx) error {
	id := c.Params("id")

	stream := h.config.GetStream(id)
	if stream == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Stream not found",
		})
	}

	info, err := ffmpeg.ProbeSource(stream.Upstream)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to probe source",
			"details": err.Error(),
		})
	}

	return c.JSON(info)
}

// GetStreamCharacteristics probes the upstream source to detect stream characteristics
func (h *StreamHandler) GetStreamCharacteristics(c *fiber.Ctx) error {
	id := c.Params("id")

	stream := h.config.GetStream(id)
	if stream == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Stream not found",
		})
	}

	chars, err := ffmpeg.ProbeStreamCharacteristics(stream.Upstream)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to probe stream characteristics",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"stream_type":     string(chars.StreamType),
		"segment_format":  string(chars.SegmentFormat),
		"is_multi_variant": chars.IsMultiVariant,
		"has_subtitles":   chars.HasSubtitles,
		"has_audio":       chars.HasAudio,
		"target_duration": chars.TargetDuration,
		"max_bandwidth":   chars.MaxBandwidth,
		"max_resolution":  chars.MaxResolution,
		"variant_count":   chars.VariantCount,
	})
}
