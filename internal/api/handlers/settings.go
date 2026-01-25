package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/models"
	"github.com/ParkWardRR/relaystation/internal/stream"
)

// SettingsHandler handles settings-related API requests
type SettingsHandler struct {
	config  *config.Manager
	manager *stream.Manager
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(cfg *config.Manager, mgr *stream.Manager) *SettingsHandler {
	return &SettingsHandler{config: cfg, manager: mgr}
}

// GetDefaults returns the global default settings
func (h *SettingsHandler) GetDefaults(c *fiber.Ctx) error {
	defaults := h.config.GetDefaults()
	return c.JSON(defaults)
}

// UpdateDefaults updates the global default settings
func (h *SettingsHandler) UpdateDefaults(c *fiber.Ctx) error {
	var defaults models.Defaults
	if err := c.BodyParser(&defaults); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate
	if defaults.SegmentTime <= 0 {
		defaults.SegmentTime = 2
	}
	if defaults.PlaylistSize <= 0 {
		defaults.PlaylistSize = 6
	}
	if defaults.Preset == "" {
		defaults.Preset = "ultrafast"
	}

	h.config.UpdateDefaults(defaults)

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	return c.JSON(defaults)
}

// Reload reloads the configuration and restarts streams
func (h *SettingsHandler) Reload(c *fiber.Ctx) error {
	if err := h.manager.Reload(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to reload",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Configuration reloaded",
	})
}
