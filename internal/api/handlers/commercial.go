package handlers

import (
	"github.com/ParkWardRR/relaystation/internal/commercial"
	"github.com/gofiber/fiber/v2"
)

// CommercialHandler provides HTTP handlers for the commercial detector API.
type CommercialHandler struct {
	detector *commercial.Detector
	db       *commercial.PatternDB
}

// NewCommercialHandler creates a new handler for commercial detection endpoints.
func NewCommercialHandler(det *commercial.Detector, db *commercial.PatternDB) *CommercialHandler {
	return &CommercialHandler{detector: det, db: db}
}

// GetStatus returns the current commercial detection status (GET /api/commercial/status).
func (h *CommercialHandler) GetStatus(c *fiber.Ctx) error {
	status := h.detector.GetStatus()
	return c.JSON(status)
}

// GetPatterns returns the pattern database statistics (GET /api/commercial/patterns).
func (h *CommercialHandler) GetPatterns(c *fiber.Ctx) error {
	stats := h.db.Stats()
	return c.JSON(stats)
}

// IsCommercial returns a simple boolean check (GET /api/commercial/check).
func (h *CommercialHandler) IsCommercial(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"commercial": h.detector.IsCommercial(),
		"state":      h.detector.GetStatus().State,
	})
}
