<div align="center">

# RelayStation

**Self-hosted HLS streaming relay and transcoder with a modern web interface**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Blue_Oak_1.0.0-4A90D9?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-Frontend-FF3E00?style=for-the-badge&logo=svelte&logoColor=white)](https://kit.svelte.dev/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind-CSS-06B6D4?style=for-the-badge&logo=tailwindcss&logoColor=white)](https://tailwindcss.com/)
[![FFmpeg](https://img.shields.io/badge/FFmpeg-Powered-007808?style=for-the-badge&logo=ffmpeg&logoColor=white)](https://ffmpeg.org/)

[![GitHub Release](https://img.shields.io/github/v/release/ParkWardRR/relaystation?style=for-the-badge&logo=github)](https://github.com/ParkWardRR/relaystation/releases)
[![GitHub Stars](https://img.shields.io/github/stars/ParkWardRR/relaystation?style=for-the-badge&logo=github)](https://github.com/ParkWardRR/relaystation/stargazers)
[![PRs Welcome](https://img.shields.io/badge/PRs-Welcome-brightgreen?style=for-the-badge)](https://github.com/ParkWardRR/relaystation/pulls)

---

[Features](#features) • [Installation](#installation) • [Configuration](#configuration) • [API](#api-endpoints) • [Development](#development)

</div>

![Dashboard](screenshots/dashboard.png)

## Features

- **Real-time HLS Transcoding** — Convert any HLS stream to H.264/H.265 with configurable quality
- **8 Built-in Presets** — Optimized for different devices (iPad, Apple TV, mobile, etc.)
- **Live Status Dashboard** — Real-time stream monitoring via WebSocket
- **Modern Admin Panel** — Manage streams, presets, and settings with a clean UI
- **Docker-Ready** — Single-command deployment with Docker Compose
- **Source Probing** — Automatically detect upstream stream quality and variants

## Screenshots

<details>
<summary><strong>Admin Panel - Streams</strong></summary>

![Admin Streams](screenshots/admin-streams.png)
</details>

<details>
<summary><strong>Admin Panel - Presets</strong></summary>

![Admin Presets](screenshots/admin-presets.png)
</details>

<details>
<summary><strong>Admin Panel - Settings</strong></summary>

![Admin Settings](screenshots/admin-settings.png)
</details>

## Tech Stack

| Component | Technology |
|-----------|------------|
| **Backend** | Go 1.22 + Fiber |
| **Frontend** | SvelteKit + Tailwind CSS + shadcn-svelte |
| **Streaming** | FFmpeg with H.264/H.265 |
| **Deployment** | Docker + Docker Compose |

---

## Installation

Choose your platform below for detailed setup instructions.

### 🍎 macOS + OrbStack (Development)

> Perfect for local development and testing with OrbStack's lightweight Docker environment.

**Prerequisites:**
- [OrbStack](https://orbstack.dev/) installed
- Git

```bash
# Clone the repository
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation

# Start with Docker Compose (OrbStack handles Docker automatically)
docker compose -f docker/docker-compose.yml up -d

# View logs
docker compose -f docker/docker-compose.yml logs -f

# Access the dashboard
open http://localhost:8080
```

**Development Mode (with hot reload):**

```bash
# Install prerequisites via Homebrew
brew install go node ffmpeg

# Build and run locally
make web-install
make web-build
make dev

# Or use development Docker Compose
docker compose -f docker/docker-compose.dev.yml up
```

---

### 🐧 AlmaLinux (Production Server)

> Recommended for production deployments on RHEL-compatible systems.

**Step 1: Install Dependencies**

```bash
# Update system
sudo dnf update -y

# Install Docker
sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Start and enable Docker
sudo systemctl start docker
sudo systemctl enable docker

# Add your user to docker group (logout/login required)
sudo usermod -aG docker $USER
```

**Step 2: Deploy RelayStation**

```bash
# Clone the repository
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation

# Create data directories
mkdir -p /opt/relaystation/data
mkdir -p /opt/relaystation/configs

# Copy configuration
cp configs/streams.json /opt/relaystation/configs/

# Start the application
docker compose -f docker/docker-compose.yml up -d

# Enable auto-start on boot
docker update --restart unless-stopped relaystation
```

**Step 3: Configure Firewall**

```bash
# Allow HTTP traffic
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

**Step 4: (Optional) Setup with systemd**

Create `/etc/systemd/system/relaystation.service`:

```ini
[Unit]
Description=RelayStation HLS Streaming Relay
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/relaystation
ExecStart=/usr/bin/docker compose -f docker/docker-compose.yml up -d
ExecStop=/usr/bin/docker compose -f docker/docker-compose.yml down

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable relaystation
sudo systemctl start relaystation
```

---

### 🌊 DigitalOcean Droplet

> Quick deployment on DigitalOcean with Docker pre-installed.

**Step 1: Create Droplet**

1. Log in to [DigitalOcean](https://cloud.digitalocean.com/)
2. Create a new Droplet:
   - **Image:** Docker on Ubuntu (from Marketplace)
   - **Size:** Basic $6/mo (1 vCPU, 1GB RAM) or higher for multiple streams
   - **Region:** Choose closest to your users
   - **Authentication:** SSH keys (recommended)

**Step 2: Connect and Deploy**

```bash
# SSH into your droplet
ssh root@your-droplet-ip

# Clone the repository
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation

# Start RelayStation
docker compose -f docker/docker-compose.yml up -d

# Verify it's running
docker compose -f docker/docker-compose.yml ps
```

**Step 3: Configure UFW Firewall**

```bash
# Allow SSH (important!)
ufw allow OpenSSH

# Allow RelayStation
ufw allow 8080/tcp

# Enable firewall
ufw enable
```

**Step 4: (Optional) Setup Domain with Nginx**

```bash
# Install Nginx
apt update && apt install -y nginx certbot python3-certbot-nginx

# Create Nginx config
cat > /etc/nginx/sites-available/relaystation << 'EOF'
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_cache_bypass $http_upgrade;
    }
}
EOF

# Enable site
ln -s /etc/nginx/sites-available/relaystation /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

# Get SSL certificate
certbot --nginx -d your-domain.com
```

---

## Configuration

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

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `CONFIG_PATH` | `configs/streams.json` | Path to stream configuration |
| `HLS_OUTPUT_DIR` | `./hls` | Directory for HLS segments |
| `FFMPEG_PATH` | `ffmpeg` | Path to FFmpeg binary |

---

## Built-in Presets

| Preset | Codec | Resolution | Bitrate | Use Case |
|--------|-------|------------|---------|----------|
| **HEVC 1080p** | H.265 | 1920x1080 | 4.0 Mbps | Modern Apple devices |
| **HEVC Max Quality** | H.265 | 1920x1080 | 8.0 Mbps | Apple TV 4K, iPad Pro |
| **Legacy iPad** | H.264 | 1280x720 | 2.0 Mbps | iPad 2, iPad Mini 1-3, iPad 3-4 |
| **Modern Tablet** | H.264 | 1920x1080 | 4.0 Mbps | iPad Air, iPad Pro, iPad Mini 4+ |
| **Apple TV 3rd Gen** | H.264 | 1280x720 | 2.5 Mbps | Apple TV 3 (A1427/A1469) |
| **Mobile Optimized** | H.264 | 854x480 | 1.0 Mbps | Low bandwidth / cellular |
| **High Quality** | H.264 | 1920x1080 | 6.0 Mbps | Fast networks, max H.264 quality |
| **Passthrough** | Copy | — | — | No transcoding, lowest latency |

---

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

---

## Development

### Prerequisites

| Requirement | Version |
|-------------|---------|
| Go | 1.22+ |
| Node.js | 20+ |
| FFmpeg | 6.0+ |

### Build from Source

```bash
# Clone
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation

# Backend
go build -o bin/relaystation ./cmd/relaystation

# Frontend
cd web && npm install && npm run build
```

### Make Commands

```bash
make build        # Build Go binary
make dev          # Run in development mode
make test         # Run tests
make clean        # Clean build artifacts
make docker       # Build Docker image
make docker-up    # Start with Docker Compose
make docker-down  # Stop Docker Compose
make docker-dev   # Development with Docker Compose
make web-install  # Install web dependencies
make web-build    # Build web frontend
make web-dev      # Run web dev server
make all          # Full build (web + docker)
```

---

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

---

## License

This project is licensed under the [Blue Oak Model License 1.0.0](LICENSE).

---

<div align="center">

**[⬆ Back to Top](#relaystation)**

Made with ❤️ for the streaming community

</div>
