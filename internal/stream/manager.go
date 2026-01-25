package stream

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ParkWardRR/relaystation/internal/config"
	"github.com/ParkWardRR/relaystation/internal/ffmpeg"
	"github.com/ParkWardRR/relaystation/internal/models"
)

// Manager orchestrates all FFmpeg transcoding processes
type Manager struct {
	config     *config.Manager
	processes  map[string]*ffmpeg.Process
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	outputBase string
	eventChan  chan models.StreamEvent
	startTime  time.Time
}

// NewManager creates a new stream manager
func NewManager(cfg *config.Manager, outputBase string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:     cfg,
		processes:  make(map[string]*ffmpeg.Process),
		ctx:        ctx,
		cancel:     cancel,
		outputBase: outputBase,
		eventChan:  make(chan models.StreamEvent, 100),
		startTime:  time.Now(),
	}
}

// Start begins the stream manager
func (m *Manager) Start() {
	log.Println("[StreamManager] Starting...")

	// Initial sync
	m.Sync()

	// Watch for config changes
	m.config.SetOnChange(func() {
		log.Println("[StreamManager] Config changed, resyncing...")
		m.Sync()
	})

	// Start health check loop
	m.wg.Add(1)
	go m.healthCheckLoop()

	log.Println("[StreamManager] Started successfully")
}

// Stop shuts down all processes
func (m *Manager) Stop() {
	log.Println("[StreamManager] Stopping...")

	m.cancel()

	m.mu.Lock()
	for id, proc := range m.processes {
		log.Printf("[StreamManager] Stopping process %s", id)
		proc.Stop()
	}
	m.processes = make(map[string]*ffmpeg.Process)
	m.mu.Unlock()

	m.wg.Wait()
	close(m.eventChan)

	log.Println("[StreamManager] Stopped")
}

// Sync synchronizes running processes with configuration
func (m *Manager) Sync() {
	streams := m.config.GetStreams()
	defaults := m.config.GetDefaults()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Track which processes should exist
	desired := make(map[string]bool)

	// Start new processes for enabled streams/profiles
	for _, stream := range streams {
		if !stream.Enabled {
			continue
		}

		for profileKey, profile := range stream.Profiles {
			if !profile.Enabled || profile.Passthrough {
				continue
			}

			processID := stream.ID + "_" + profileKey

			desired[processID] = true

			// Check if process already exists
			if _, exists := m.processes[processID]; exists {
				continue
			}

			// Create and start new process
			profile.ID = processID
			proc := ffmpeg.NewProcess(profile, stream.Upstream, m.outputBase, defaults)

			if err := proc.Start(m.ctx); err != nil {
				log.Printf("[StreamManager] Failed to start process %s: %v", processID, err)
				continue
			}

			m.processes[processID] = proc
			m.sendEvent("stream_started", stream.ID, nil)
		}
	}

	// Stop processes that should no longer exist
	for id, proc := range m.processes {
		if !desired[id] {
			log.Printf("[StreamManager] Stopping obsolete process %s", id)
			proc.Stop()
			delete(m.processes, id)
			m.sendEvent("stream_stopped", id, nil)
		}
	}
}

// Reload forces a configuration reload and process sync
func (m *Manager) Reload() error {
	if err := m.config.Load(); err != nil {
		return err
	}
	m.Sync()
	return nil
}

func (m *Manager) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkHealth()
		}
	}
}

func (m *Manager) checkHealth() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, proc := range m.processes {
		if !proc.IsRunning() {
			log.Printf("[StreamManager] Process %s is not running", id)
		}
	}
}

func (m *Manager) sendEvent(eventType, streamID string, data interface{}) {
	select {
	case m.eventChan <- models.StreamEvent{
		Type:     eventType,
		StreamID: streamID,
		Data:     data,
	}:
	default:
		// Channel full, drop event
	}
}

// GetStatus returns the current status of all streams
func (m *Manager) GetStatus() []models.StreamStatus {
	streams := m.config.GetStreams()

	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]models.StreamStatus, 0, len(streams))

	for _, stream := range streams {
		status := models.StreamStatus{
			ID:           stream.ID,
			Name:         stream.Name,
			Upstream:     stream.Upstream,
			UpstreamLive: ffmpeg.CheckUpstreamLive(stream.Upstream),
			Enabled:      stream.Enabled,
			Profiles:     make([]models.ProfileStatus, 0),
		}

		for profileKey, profile := range stream.Profiles {
			processID := stream.ID + "_" + profileKey

			profileStatus := models.ProfileStatus{
				ID:          processID,
				Path:        profile.Path,
				Passthrough: profile.Passthrough,
				Enabled:     profile.Enabled,
				Codec:       profile.Codec,
				Resolution:  profile.Resolution,
				Bitrate:     profile.Bitrate,
			}

			if proc, exists := m.processes[processID]; exists {
				profileStatus.Running = proc.IsRunning()
				profileStatus.Live = proc.IsLive()
				profileStatus.RestartCount = proc.RestartCount()
			}

			status.Profiles = append(status.Profiles, profileStatus)
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// Subscribe returns a channel for real-time events
func (m *Manager) Subscribe() <-chan models.StreamEvent {
	return m.eventChan
}

// Uptime returns the duration since the manager started
func (m *Manager) Uptime() time.Duration {
	return time.Since(m.startTime)
}

// OutputBase returns the base directory for HLS output
func (m *Manager) OutputBase() string {
	return m.outputBase
}
