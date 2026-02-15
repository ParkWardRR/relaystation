package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"

	"github.com/ParkWardRR/relaystation/internal/api/handlers"
	"github.com/ParkWardRR/relaystation/internal/commercial"
	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/relay"
	"github.com/ParkWardRR/relaystation/internal/stream"
)

// NewRouter creates and configures the Fiber application
func NewRouter(cfg *config.Manager, mgr *stream.Manager, relayInstance *relay.Relay, detector *commercial.Detector, patternDB *commercial.PatternDB, staticDir string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "RelayStation",
	})

	// Middleware
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Create handlers
	streamHandler := handlers.NewStreamHandler(cfg, mgr)
	presetHandler := handlers.NewPresetHandler(cfg)
	statusHandler := handlers.NewStatusHandler(cfg, mgr)
	settingsHandler := handlers.NewSettingsHandler(cfg, mgr)
	wsHandler := handlers.NewWebSocketHandler(mgr)
	relayHandler := handlers.NewRelayHandler(relayInstance)
	commercialHandler := handlers.NewCommercialHandler(detector, patternDB)

	// API routes
	api := app.Group("/api")

	// Status
	api.Get("/status", statusHandler.GetStatus)
	api.Get("/health", statusHandler.Health)

	// Streams
	api.Get("/streams", streamHandler.List)
	api.Post("/streams", streamHandler.Create)
	api.Get("/streams/:id", streamHandler.Get)
	api.Put("/streams/:id", streamHandler.Update)
	api.Delete("/streams/:id", streamHandler.Delete)
	api.Put("/streams/:id/preset", streamHandler.ApplyPreset)
	api.Get("/streams/:id/source-info", streamHandler.GetSourceInfo)
	api.Get("/streams/:id/characteristics", streamHandler.GetStreamCharacteristics)
	api.Post("/probe-url", streamHandler.ProbeURL)

	// Presets
	api.Get("/presets", presetHandler.List)
	api.Post("/presets", presetHandler.Create)
	api.Delete("/presets/:id", presetHandler.Delete)

	// Relay
	api.Get("/relay/status", relayHandler.GetStatus)
	api.Post("/relay/switch/:idx", relayHandler.SwitchSource)
	api.Post("/relay/scan", relayHandler.ScanSources)

	// Commercial Detection
	api.Get("/commercial/status", commercialHandler.GetStatus)
	api.Get("/commercial/check", commercialHandler.IsCommercial)
	api.Get("/commercial/patterns", commercialHandler.GetPatterns)

	// Relay Dashboard (before SPA fallback)
	app.Get("/relay", relayHandler.Dashboard)

	// Settings
	api.Get("/defaults", settingsHandler.GetDefaults)
	api.Put("/defaults", settingsHandler.UpdateDefaults)
	api.Post("/reload", settingsHandler.Reload)

	// WebSocket - check if upgrade request
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/events", wsHandler.Upgrade())

	// HLS output files - serve from manager's output directory
	hlsOutputDir := mgr.OutputBase()
	app.Static("/hls", hlsOutputDir, fiber.Static{
		Compress:      false,
		CacheDuration: 0,
	})

	// Static files (SvelteKit build)
	if staticDir != "" {
		app.Static("/", staticDir)

		// SPA fallback - serve index.html for non-API routes
		app.Get("/*", func(c *fiber.Ctx) error {
			return c.SendFile(staticDir + "/index.html")
		})
	}

	return app
}
