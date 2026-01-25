package models

// Stream represents a source stream with multiple output profiles
type Stream struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Upstream string              `json:"upstream"`
	Enabled  bool                `json:"enabled"`
	Profiles map[string]*Profile `json:"profiles"`
}

// Profile represents a single output transcoding profile
type Profile struct {
	ID           string `json:"id,omitempty"`
	Enabled      bool   `json:"enabled"`
	Passthrough  bool   `json:"passthrough"`
	Path         string `json:"path"`
	Codec        string `json:"codec,omitempty"`
	ProfileName  string `json:"profile,omitempty"`
	Level        string `json:"level,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	Bitrate      string `json:"bitrate,omitempty"`
	MaxRate      string `json:"maxrate,omitempty"`
	FPS          int    `json:"fps,omitempty"`
	AudioBitrate string `json:"audio_bitrate,omitempty"`
	AudioSample  string `json:"audio_sample,omitempty"`
}

// Preset represents a transcoding preset (built-in or custom)
type Preset struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subtitle     string `json:"subtitle,omitempty"`
	Description  string `json:"description,omitempty"`
	Codec        string `json:"codec"`
	ProfileName  string `json:"profile"`
	Level        string `json:"level"`
	Resolution   string `json:"resolution"`
	Bitrate      string `json:"bitrate"`
	MaxRate      string `json:"maxrate"`
	FPS          int    `json:"fps"`
	AudioBitrate string `json:"audio_bitrate"`
	AudioSample  string `json:"audio_sample"`
	Builtin      bool   `json:"builtin"`
}

// Defaults represents global configuration defaults
type Defaults struct {
	SegmentTime  int    `json:"segment_time"`
	PlaylistSize int    `json:"playlist_size"`
	Preset       string `json:"preset"`
}

// Config represents the full configuration file
type Config struct {
	Streams       []Stream          `json:"streams"`
	Defaults      Defaults          `json:"defaults"`
	CustomPresets map[string]Preset `json:"custom_presets"`
}

// StreamStatus represents the runtime status of a stream
type StreamStatus struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Upstream     string          `json:"upstream"`
	UpstreamLive bool            `json:"upstream_live"`
	Enabled      bool            `json:"enabled"`
	Profiles     []ProfileStatus `json:"profiles"`
}

// ProfileStatus represents the runtime status of a profile
type ProfileStatus struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Passthrough  bool   `json:"passthrough"`
	Enabled      bool   `json:"enabled"`
	Live         bool   `json:"live"`
	Running      bool   `json:"running"`
	RestartCount int    `json:"restart_count"`
	Codec        string `json:"codec"`
	Resolution   string `json:"resolution"`
	Bitrate      string `json:"bitrate"`
}

// ServerInfo represents server metadata
type ServerInfo struct {
	Hostname   string `json:"hostname"`
	PublicIP   string `json:"public_ip,omitempty"`
	ReverseDNS string `json:"reverse_dns,omitempty"`
	Uptime     string `json:"uptime"`
	Version    string `json:"version"`
}

// StatusResponse is the response for /api/status
type StatusResponse struct {
	Streams []StreamStatus `json:"streams"`
	Server  ServerInfo     `json:"server"`
}

// StreamEvent represents a real-time event for WebSocket
type StreamEvent struct {
	Type     string      `json:"type"`
	StreamID string      `json:"stream_id,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// SourceVariant represents a quality variant from source probing
type SourceVariant struct {
	Bandwidth  int    `json:"bandwidth"`
	Resolution string `json:"resolution"`
	Codecs     string `json:"codecs"`
	URI        string `json:"uri,omitempty"`
}

// SourceInfo represents probed source information
type SourceInfo struct {
	Variants   []SourceVariant `json:"variants"`
	MaxQuality string          `json:"max_quality"`
}
