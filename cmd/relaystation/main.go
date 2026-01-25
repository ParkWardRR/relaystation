package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ParkWardRR/relaystation/internal/api"
	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/stream"
)

func main() {
	log.Println("RelayStation - HLS Streaming Relay")
	log.Println("===================================")

	// Configuration
	configPath := getEnv("RELAYSTATION_CONFIG", "/etc/relaystation/streams.json")
	outputBase := getEnv("RELAYSTATION_OUTPUT", "/var/www/hls")
	staticDir := getEnv("RELAYSTATION_STATIC", "./web/build")
	listenAddr := getEnv("RELAYSTATION_ADDR", ":8080")

	log.Printf("Config: %s", configPath)
	log.Printf("Output: %s", outputBase)
	log.Printf("Static: %s", staticDir)
	log.Printf("Listen: %s", listenAddr)

	// Ensure output directory exists
	if err := os.MkdirAll(outputBase, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Load configuration
	cfg := config.NewManager(configPath)
	if err := cfg.Load(); err != nil {
		log.Printf("Warning: Could not load config: %v", err)
		log.Println("Starting with empty configuration...")
	}

	// Start watching for config changes
	if err := cfg.Watch(); err != nil {
		log.Printf("Warning: Could not watch config file: %v", err)
	}

	// Create stream manager
	mgr := stream.NewManager(cfg, outputBase)
	mgr.Start()

	// Create API router
	app := api.NewRouter(cfg, mgr, staticDir)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		mgr.Stop()
		app.Shutdown()
	}()

	// Start server
	log.Printf("Starting server on %s", listenAddr)
	if err := app.Listen(listenAddr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
