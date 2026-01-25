package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/models"
)

// PresetHandler handles preset-related API requests
type PresetHandler struct {
	config *config.Manager
}

// NewPresetHandler creates a new preset handler
func NewPresetHandler(cfg *config.Manager) *PresetHandler {
	return &PresetHandler{config: cfg}
}

// List returns all presets (built-in + custom)
func (h *PresetHandler) List(c *fiber.Ctx) error {
	presets := h.config.GetAllPresets()
	return c.JSON(presets)
}

// Create adds a new custom preset
func (h *PresetHandler) Create(c *fiber.Ctx) error {
	var preset models.Preset
	if err := c.BodyParser(&preset); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if preset.ID == "" || preset.Name == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "ID and name are required",
		})
	}

	// Check if preset ID already exists (built-in or custom)
	if _, ok := config.BuiltinPresets[preset.ID]; ok {
		return c.Status(409).JSON(fiber.Map{
			"error": "Cannot override built-in preset",
		})
	}

	h.config.AddCustomPreset(preset)

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	return c.Status(201).JSON(preset)
}

// Delete removes a custom preset
func (h *PresetHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	// Check if it's a built-in preset
	if _, ok := config.BuiltinPresets[id]; ok {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot delete built-in preset",
		})
	}

	if !h.config.DeleteCustomPreset(id) {
		return c.Status(404).JSON(fiber.Map{
			"error": "Preset not found",
		})
	}

	if err := h.config.Save(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Preset deleted",
	})
}
