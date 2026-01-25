package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ParkWardRR/relaystation/internal/models"
	"github.com/fsnotify/fsnotify"
)

// Manager handles configuration loading and watching
type Manager struct {
	path       string
	config     *models.Config
	mu         sync.RWMutex
	onChange   func()
	lastModify time.Time
}

// NewManager creates a new config manager
func NewManager(configPath string) *Manager {
	return &Manager{
		path: configPath,
	}
}

// Load reads the configuration from disk
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}

	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Set defaults if not specified
	if cfg.Defaults.SegmentTime == 0 {
		cfg.Defaults.SegmentTime = 2
	}
	if cfg.Defaults.PlaylistSize == 0 {
		cfg.Defaults.PlaylistSize = 6
	}
	if cfg.Defaults.Preset == "" {
		cfg.Defaults.Preset = "ultrafast"
	}
	if cfg.CustomPresets == nil {
		cfg.CustomPresets = make(map[string]models.Preset)
	}

	m.config = &cfg
	return nil
}

// Save writes the configuration to disk
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

// Get returns the current configuration
func (m *Manager) Get() *models.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetStreams returns all streams
func (m *Manager) GetStreams() []models.Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return nil
	}
	return m.config.Streams
}

// GetStream returns a specific stream by ID
func (m *Manager) GetStream(id string) *models.Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil
	}

	for i := range m.config.Streams {
		if m.config.Streams[i].ID == id {
			return &m.config.Streams[i]
		}
	}
	return nil
}

// AddStream adds a new stream
func (m *Manager) AddStream(stream models.Stream) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Streams = append(m.config.Streams, stream)
	return nil
}

// UpdateStream updates an existing stream
func (m *Manager) UpdateStream(id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.config.Streams {
		if m.config.Streams[i].ID == id {
			if name, ok := updates["name"].(string); ok {
				m.config.Streams[i].Name = name
			}
			if upstream, ok := updates["upstream"].(string); ok {
				m.config.Streams[i].Upstream = upstream
			}
			if enabled, ok := updates["enabled"].(bool); ok {
				m.config.Streams[i].Enabled = enabled
			}
			return nil
		}
	}
	return nil
}

// DeleteStream removes a stream by ID
func (m *Manager) DeleteStream(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.config.Streams {
		if m.config.Streams[i].ID == id {
			m.config.Streams = append(m.config.Streams[:i], m.config.Streams[i+1:]...)
			return nil
		}
	}
	return nil
}

// GetDefaults returns the default settings
func (m *Manager) GetDefaults() models.Defaults {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return models.Defaults{
			SegmentTime:  2,
			PlaylistSize: 6,
			Preset:       "ultrafast",
		}
	}
	return m.config.Defaults
}

// UpdateDefaults updates the default settings
func (m *Manager) UpdateDefaults(defaults models.Defaults) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Defaults = defaults
}

// GetAllPresets returns all presets (built-in + custom)
func (m *Manager) GetAllPresets() []models.Preset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	customCount := 0
	if m.config != nil {
		customCount = len(m.config.CustomPresets)
	}

	presets := make([]models.Preset, 0, len(BuiltinPresets)+customCount)

	// Add built-in presets first
	for _, p := range BuiltinPresets {
		presets = append(presets, p)
	}

	// Add custom presets
	if m.config != nil {
		for _, p := range m.config.CustomPresets {
			presets = append(presets, p)
		}
	}

	return presets
}

// AddCustomPreset adds a custom preset
func (m *Manager) AddCustomPreset(preset models.Preset) {
	m.mu.Lock()
	defer m.mu.Unlock()

	preset.Builtin = false
	if m.config.CustomPresets == nil {
		m.config.CustomPresets = make(map[string]models.Preset)
	}
	m.config.CustomPresets[preset.ID] = preset
}

// DeleteCustomPreset removes a custom preset
func (m *Manager) DeleteCustomPreset(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cannot delete built-in presets
	if _, ok := BuiltinPresets[id]; ok {
		return false
	}

	delete(m.config.CustomPresets, id)
	return true
}

// SetOnChange sets a callback for configuration changes
func (m *Manager) SetOnChange(fn func()) {
	m.onChange = fn
}

// Watch starts watching the config file for changes
func (m *Manager) Watch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					if err := m.Load(); err == nil && m.onChange != nil {
						m.onChange()
					}
				}
			case <-watcher.Errors:
				// Ignore errors
			}
		}
	}()

	return watcher.Add(m.path)
}

// ApplyPresetToStream applies a preset to a stream's profile
func (m *Manager) ApplyPresetToStream(streamID string, preset models.Preset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.config.Streams {
		if m.config.Streams[i].ID == streamID {
			profileID := streamID + "_" + preset.ID
			profile := &models.Profile{
				ID:           profileID,
				Enabled:      true,
				Passthrough:  preset.ID == "passthrough",
				Path:         "/" + preset.ID + "/" + streamID + "/",
				Codec:        preset.Codec,
				ProfileName:  preset.ProfileName,
				Level:        preset.Level,
				Resolution:   preset.Resolution,
				Bitrate:      preset.Bitrate,
				MaxRate:      preset.MaxRate,
				FPS:          preset.FPS,
				AudioBitrate: preset.AudioBitrate,
				AudioSample:  preset.AudioSample,
			}

			if m.config.Streams[i].Profiles == nil {
				m.config.Streams[i].Profiles = make(map[string]*models.Profile)
			}
			m.config.Streams[i].Profiles[preset.ID] = profile
			return nil
		}
	}
	return nil
}
