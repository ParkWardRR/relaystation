package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ParkWardRR/relaystation/internal/api"
	"github.com/ParkWardRR/relaystation/internal/commercial"
	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/relay"
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

	// Create IMSA failover relay with DO droplet sources
	imsaRelay := relay.NewRelay(relay.Config{
		Sources: []relay.FeedSource{
			{URL: "http://209.38.25.42/live/imsa_international/master.m3u8", Label: "IMSA-International", Preferred: true},
			{URL: "http://209.38.25.42/live/imsa_icc_01/master.m3u8", Label: "IMSA-ICC1"},
			{URL: "http://209.38.25.42/live/imsa_icc_02/master.m3u8", Label: "IMSA-ICC2"},
		},
		OutputBase:  outputBase,
		OutputPath:  "relay/imsa",
		ListenAddr:  listenAddr,
		MaxRestarts: 3,
	})

	// Start the relay
	if err := imsaRelay.Start(); err != nil {
		log.Printf("Warning: Could not start IMSA relay: %v", err)
		log.Println("Relay will be available at /api/relay/status once sources are live")
	} else {
		log.Println("===========================================")
		log.Println("IMSA Relay is LIVE!")
		log.Printf("VLC URL:       http://localhost%s/hls/relay/imsa/stream.m3u8", listenAddr)
		log.Printf("Dashboard:     http://localhost%s/relay", listenAddr)
		log.Println("===========================================")
	}
	// ──── Commercial Detector ────
	// Monitor IMSA-International for silence to detect commercial breaks.
	patternDBPath := filepath.Join(outputBase, "commercial_patterns.json")
	patternDB := commercial.NewPatternDB(patternDBPath)

	commDetector := commercial.NewDetector(commercial.Config{
		StreamURL:          "http://209.38.25.42/live/imsa_international/master.m3u8",
		StreamLabel:        "IMSA-International",
		SilenceThresholdDB: -30,
		CommercialSec:      10,
		ClassifierPath:     "./tools/soundclassifier/soundclassifier",
	})

	// Learner captures fingerprints from other streams during commercials
	commLearner := commercial.NewLearner(commDetector, patternDB, []commercial.LearnStream{
		{URL: "http://209.38.25.42/live/imsa_icc_01/master.m3u8", Label: "IMSA-ICC1"},
		{URL: "http://209.38.25.42/live/imsa_icc_02/master.m3u8", Label: "IMSA-ICC2"},
	})

	if err := commDetector.Start(); err != nil {
		log.Printf("Warning: Could not start commercial detector: %v", err)
	} else {
		commLearner.Start()
		log.Println("📺 Commercial detector + learner active on IMSA-International")
	}

	icc1Monitor := commercial.NewStreamMonitor(patternDB, commercial.LearnStream{
		URL: "http://209.38.25.42/live/imsa_icc_01/master.m3u8", Label: "IMSA-ICC1",
	})
	icc2Monitor := commercial.NewStreamMonitor(patternDB, commercial.LearnStream{
		URL: "http://209.38.25.42/live/imsa_icc_02/master.m3u8", Label: "IMSA-ICC2",
	})
	icc1Monitor.Start()
	icc2Monitor.Start()

	// Create API router
	app := api.NewRouter(cfg, mgr, imsaRelay, commDetector, patternDB, staticDir)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		imsaRelay.Stop()
		icc1Monitor.Stop()
		icc2Monitor.Stop()
		commLearner.Stop()
		commDetector.Stop()
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
