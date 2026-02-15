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

	// Create NASCAR failover relay with all discovered sources
	// Sources are ordered by reliability - CDN feeds first, then fallbacks
	nascarRelay := relay.NewRelay(relay.Config{
		Sources: []relay.FeedSource{
			// DD12 US Feed (primary CDN)
			{URL: "https://dtmf21.b-cdn.net/pdfs/master.m3u8", Label: "DD12-US"},
			// DD12 International Feed (CDN)
			{URL: "https://dtmf21.b-cdn.net/docx/master.m3u8", Label: "DD12-INT"},
			// AcezTrims FOX feed
			{URL: "https://bozztv.com/dvrfl05/gin-fox5/index.m3u8", Label: "ACE-FOX"},
			// AcezTrims TSN feed
			{URL: "https://stream.decentdoubts.net/809/index.m3u8", Label: "ACE-TSN"},
			// DD12 fallbacks
			{URL: "https://inshallah.woah-patio.one/hls/stream.m3u8", Label: "DD12-FB1"},
			{URL: "https://dickrida.dd12streams.com/hls/nda.m3u8", Label: "DD12-FB2"},
			{URL: "https://dd2zjam465om0.cloudfront.net/out/v1/46c8ebab8e694683b10f5b14fb0baa85/index.m3u8", Label: "DD12-CF"},
		},
		OutputBase:  outputBase,
		OutputPath:  "relay/nascar",
		ListenAddr:  listenAddr,
		MaxRestarts: 3,
	})

	// Start the relay
	if err := nascarRelay.Start(); err != nil {
		log.Printf("Warning: Could not start NASCAR relay: %v", err)
		log.Println("Relay will be available at /api/relay/status once sources are live")
	} else {
		log.Println("===========================================")
		log.Println("NASCAR Relay is LIVE!")
		log.Printf("VLC URL:       http://YOUR_LOCAL_IP%s/hls/relay/nascar/stream.m3u8", listenAddr)
		log.Printf("Dashboard:     http://YOUR_LOCAL_IP%s/relay", listenAddr)
		log.Println("===========================================")
	}
	// ──── Commercial Detector ────
	// Monitor DD12-INT for silence to detect commercial breaks.
	// When DD12-INT goes silent for 10s+, the learner captures audio
	// fingerprints from other streams, building a model that can
	// eventually detect commercials on ANY stream independently.
	patternDBPath := filepath.Join(outputBase, "commercial_patterns.json")
	patternDB := commercial.NewPatternDB(patternDBPath)

	commDetector := commercial.NewDetector(commercial.Config{
		StreamURL:          "https://dtmf21.b-cdn.net/docx/master.m3u8",
		StreamLabel:        "DD12-INT",
		SilenceThresholdDB: -30,
		CommercialSec:      10,
		ClassifierPath:     "./tools/soundclassifier/soundclassifier",
	})

	// Learner captures fingerprints from other streams during commercials
	commLearner := commercial.NewLearner(commDetector, patternDB, []commercial.LearnStream{
		{URL: "https://bozztv.com/dvrfl05/gin-fox5/index.m3u8", Label: "ACE-FOX"},
		{URL: "https://stream.decentdoubts.net/809/index.m3u8", Label: "ACE-TSN"},
		{URL: "https://dtmf21.b-cdn.net/pdfs/master.m3u8", Label: "DD12-US"},
	})

	if err := commDetector.Start(); err != nil {
		log.Printf("Warning: Could not start commercial detector: %v", err)
	} else {
		commLearner.Start()
		log.Println("📺 Commercial detector + learner active on DD12-INT")
	}

	// Independent stream monitors — detect commercials WITHOUT DD12-INT
	// These use learned patterns from the PatternDB to predict commercials
	// on any stream, so once enough data is collected, DD12-INT isn't needed.
	foxMonitor := commercial.NewStreamMonitor(patternDB, commercial.LearnStream{
		URL: "https://bozztv.com/dvrfl05/gin-fox5/index.m3u8", Label: "ACE-FOX",
	})
	tsnMonitor := commercial.NewStreamMonitor(patternDB, commercial.LearnStream{
		URL: "https://stream.decentdoubts.net/809/index.m3u8", Label: "ACE-TSN",
	})
	foxMonitor.Start()
	tsnMonitor.Start()

	// Create API router
	app := api.NewRouter(cfg, mgr, nascarRelay, commDetector, patternDB, staticDir)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		nascarRelay.Stop()
		foxMonitor.Stop()
		tsnMonitor.Stop()
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
