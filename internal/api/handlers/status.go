package handlers

import (
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/models"
	"github.com/ParkWardRR/relaystation/internal/stream"
)

// Version is set at build time
var Version = "dev"

// StatusHandler handles status-related API requests
type StatusHandler struct {
	config  *config.Manager
	manager *stream.Manager
}

// NewStatusHandler creates a new status handler
func NewStatusHandler(cfg *config.Manager, mgr *stream.Manager) *StatusHandler {
	return &StatusHandler{config: cfg, manager: mgr}
}

// GetStatus returns the current status of all streams
func (h *StatusHandler) GetStatus(c *fiber.Ctx) error {
	streams := h.manager.GetStatus()

	hostname, _ := os.Hostname()
	uptime := h.manager.Uptime()

	response := models.StatusResponse{
		Streams: streams,
		Server: models.ServerInfo{
			Hostname: hostname,
			Uptime:   formatDuration(uptime),
			Version:  Version,
		},
	}

	return c.JSON(response)
}

// Health returns a simple health check
func (h *StatusHandler) Health(c *fiber.Ctx) error {
	return c.SendString("OK")
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
