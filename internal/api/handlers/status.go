package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
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
	publicIP := getPublicIP()
	reverseDNS := getReverseDNS(publicIP)

	response := models.StatusResponse{
		Streams: streams,
		Server: models.ServerInfo{
			Hostname:   hostname,
			PublicIP:   publicIP,
			ReverseDNS: reverseDNS,
			Uptime:     formatDuration(uptime),
			Version:    Version,
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

// getPublicIP fetches the public IP address using an external service
func getPublicIP() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

// getReverseDNS performs a reverse DNS lookup for the given IP
func getReverseDNS(ip string) string {
	if ip == "" {
		return ""
	}
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	// Remove trailing dot from DNS name
	return strings.TrimSuffix(names[0], ".")
}
