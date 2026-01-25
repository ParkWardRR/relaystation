# RelayStation

Self-hosted HLS streaming relay and transcoder with a modern web interface.

![Dashboard](screenshots/dashboard.png)

## Features

- **Real-time HLS Transcoding** - Convert any HLS stream to H.264/H.265 with configurable quality
- **8 Built-in Presets** - Optimized for different devices (iPad, Apple TV, mobile, etc.)
- **Live Status Dashboard** - Real-time stream monitoring via WebSocket
- **Modern Admin Panel** - Manage streams, presets, and settings with a clean UI
- **Docker-Ready** - Single-command deployment with Docker Compose
- **Source Probing** - Automatically detect upstream stream quality and variants

## Screenshots

### Admin Panel - Streams
![Admin Streams](screenshots/admin-streams.png)

### Admin Panel - Presets
![Admin Presets](screenshots/admin-presets.png)

### Admin Panel - Settings
![Admin Settings](screenshots/admin-settings.png)

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.22 + Fiber |
| Frontend | SvelteKit + Tailwind CSS + shadcn-svelte |
| Streaming | FFmpeg with H.264/H.265 |
| Deployment | Docker + Docker Compose |

## Quick Start

### Using Docker Compose

```bash
# Clone the repository
git clone https://github.com/yourusername/relaystation.git
cd relaystation

# Start the application
docker compose -f docker/docker-compose.yml up -d

# Open the dashboard
open http://localhost:8080
```

### Configuration

Edit `configs/streams.json` to add your streams:

```json
{
  "streams": [
    {
      "id": "my-stream",
      "name": "My Stream",
      "upstream": "https://example.com/stream.m3u8",
      "enabled": true,
      "profiles": {
        "legacy_ipad": {
          "enabled": true
        }
      }
    }
  ]
}
```

## Built-in Presets

| Preset | Codec | Resolution | Bitrate | Use Case |
|--------|-------|------------|---------|----------|
| HEVC 1080p | H.265 | 1920x1080 | 4.0 Mbps | Modern Apple devices |
| HEVC Max Quality | H.265 | 1920x1080 | 8.0 Mbps | Apple TV 4K, iPad Pro |
| Legacy iPad | H.264 | 1280x720 | 2.0 Mbps | iPad 2, iPad Mini 1-3, iPad 3-4 |
| Modern Tablet | H.264 | 1920x1080 | 4.0 Mbps | iPad Air, iPad Pro, iPad Mini 4+ |
| Apple TV 3rd Gen | H.264 | 1280x720 | 2.5 Mbps | Apple TV 3 (A1427/A1469) |
| Mobile Optimized | H.264 | 854x480 | 1.0 Mbps | Low bandwidth / cellular |
| High Quality | H.264 | 1920x1080 | 6.0 Mbps | Fast networks, max H.264 quality |
| Passthrough | Copy | - | - | No transcoding, lowest latency |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/status` | GET | Server and stream status |
| `/api/streams` | GET/POST | List or create streams |
| `/api/streams/:id` | GET/PUT/DELETE | Manage a stream |
| `/api/streams/:id/source-info` | GET | Probe upstream source |
| `/api/presets` | GET/POST | List or create presets |
| `/api/presets/:id` | DELETE | Delete custom preset |
| `/api/defaults` | GET/PUT | Global default settings |
| `/api/reload` | POST | Hot-reload configuration |
| `/ws/events` | WebSocket | Real-time status updates |

## Development

### Prerequisites

- Go 1.22+
- Node.js 20+
- FFmpeg

### Build from Source

```bash
# Backend
go build -o relaystation ./cmd/relaystation

# Frontend
cd web && npm install && npm run build
```

### Run in Development

```bash
# Start backend (serves API and static files)
./relaystation

# Or use Docker for development
docker compose -f docker/docker-compose.dev.yml up
```

## Architecture

```
relaystation/
├── cmd/relaystation/       # Go entry point
├── internal/
│   ├── api/                # REST API handlers
│   ├── config/             # Configuration & presets
│   ├── ffmpeg/             # FFmpeg process management
│   ├── models/             # Data types
│   └── stream/             # Stream manager
├── web/                    # SvelteKit frontend
│   ├── src/
│   │   ├── lib/            # Components & stores
│   │   └── routes/         # Pages
│   └── package.json
├── docker/                 # Docker configuration
└── configs/                # Stream configuration
```

## License

MIT License
