<div align="center">

# RelayStation

<img src="https://img.shields.io/badge/Self--Hosted-HLS%20Relay-20B2AA?style=for-the-badge&labelColor=0f172a" alt="RelayStation">

**Transform any HLS stream with real-time transcoding, a stunning dark-mode dashboard, and effortless Docker deployment**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Blue_Oak_1.0.0-4A90D9?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-Frontend-FF3E00?style=for-the-badge&logo=svelte&logoColor=white)](https://kit.svelte.dev/)

[![Release](https://img.shields.io/badge/Release-v1.0.0-blue?style=for-the-badge&logo=github)](https://github.com/ParkWardRR/relaystation/releases/tag/v1.0.0)
[![GitHub Stars](https://img.shields.io/github/stars/ParkWardRR/relaystation?style=for-the-badge&logo=github)](https://github.com/ParkWardRR/relaystation/stargazers)
[![PRs Welcome](https://img.shields.io/badge/PRs-Welcome-brightgreen?style=for-the-badge)](https://github.com/ParkWardRR/relaystation/pulls)

---

[Features](#-features) • [Quick Start](#-quick-start) • [Screenshots](#-screenshots) • [Installation](#-installation) • [API](#-api-endpoints)

</div>

<br>

<p align="center">
  <img src="screenshots/dashboard.png" alt="RelayStation Dashboard" width="100%">
</p>

<br>

## ✨ Features

<table>
<tr>
<td width="50%">

### 🎬 Streaming
- **Real-time HLS Transcoding** — H.264 & H.265/HEVC support
- **8 Built-in Presets** — Optimized for iPad, Apple TV, mobile, and more
- **Passthrough Mode** — Zero-latency relay without transcoding
- **Source Probing** — Auto-detect upstream quality and variants

</td>
<td width="50%">

### 🖥️ Dashboard
- **Built-in Video Player** — Preview streams directly in your browser
- **Real-time Updates** — WebSocket-powered live status
- **Glass Morphism UI** — Modern, responsive dark-mode design
- **Mobile-First** — Works beautifully on any device

</td>
</tr>
<tr>
<td width="50%">

### ⚙️ Management
- **Stream Manager** — Add, edit, and toggle streams on the fly
- **Preset Editor** — Customize or create new encoding profiles
- **Hot Reload** — Apply changes without restarting
- **REST API** — Full programmatic control

</td>
<td width="50%">

### 🚀 Deployment
- **Single Docker Command** — Up and running in seconds
- **Multi-Platform** — macOS, Linux, DigitalOcean, and more
- **Nginx Ready** — SSL/TLS termination support
- **Systemd Integration** — Auto-start on boot

</td>
</tr>
</table>

<br>

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation

# Start with Docker Compose
docker compose -f docker/docker-compose.yml up -d

# Open your browser
open http://localhost:8080
```

That's it! Add your first stream via the Admin Panel or edit `configs/streams.json`.

<br>

## 📸 Screenshots

<details open>
<summary><strong>🎛️ Live Dashboard</strong> — Real-time stream monitoring with built-in HLS player</summary>
<br>
<p align="center">
  <img src="screenshots/dashboard.png" alt="Dashboard" width="100%">
</p>
</details>

<details>
<summary><strong>📱 Mobile View</strong> — Fully responsive design for on-the-go monitoring</summary>
<br>
<p align="center">
  <img src="screenshots/dashboard-mobile.png" alt="Mobile Dashboard" width="300">
</p>
</details>

<details>
<summary><strong>📡 Admin Panel - Streams</strong> — Manage all your streams in one place</summary>
<br>
<p align="center">
  <img src="screenshots/admin-streams.png" alt="Admin Streams" width="100%">
</p>
</details>

<details>
<summary><strong>🎨 Admin Panel - Presets</strong> — Customize transcoding profiles</summary>
<br>
<p align="center">
  <img src="screenshots/admin-presets.png" alt="Admin Presets" width="100%">
</p>
</details>

<details>
<summary><strong>⚙️ Admin Panel - Settings</strong> — Configure global defaults</summary>
<br>
<p align="center">
  <img src="screenshots/admin-settings.png" alt="Admin Settings" width="100%">
</p>
</details>

<br>

## 🛠️ Tech Stack

| Component | Technology |
|-----------|------------|
| **Backend** | Go 1.22 + [Fiber](https://gofiber.io/) |
| **Frontend** | [SvelteKit](https://kit.svelte.dev/) + [Tailwind CSS](https://tailwindcss.com/) + [shadcn-svelte](https://www.shadcn-svelte.com/) |
| **Video Player** | [HLS.js](https://github.com/video-dev/hls.js) |
| **Streaming** | [FFmpeg](https://ffmpeg.org/) (H.264/H.265) |
| **Deployment** | Docker + Docker Compose |

<br>

## 📦 Installation

Choose your platform below. Each includes both development and production setup.

<details>
<summary><strong>🍎 macOS + OrbStack</strong></summary>

### Prerequisites
- [OrbStack](https://orbstack.dev/) (or Docker Desktop)
- Git

### Development Setup
```bash
# Install dev tools via Homebrew
brew install go node ffmpeg

# Clone the repository
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation

# Install frontend dependencies
make web-install

# Run backend + frontend separately (hot reload)
make web-dev          # Terminal 1: Frontend dev server
make dev              # Terminal 2: Backend
```

### Production Setup
```bash
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation
docker compose -f docker/docker-compose.yml up -d
open http://localhost:8080
```

**Auto-start on login:** Create `~/Library/LaunchAgents/com.relaystation.plist` — see full docs below.

</details>

<details>
<summary><strong>🐧 AlmaLinux / RHEL</strong></summary>

### Prerequisites
```bash
sudo dnf update -y
sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
```

### Production Setup
```bash
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation
docker compose -f docker/docker-compose.yml up -d
sudo firewall-cmd --permanent --add-port=8080/tcp && sudo firewall-cmd --reload
```

**Auto-start with systemd:** Create `/etc/systemd/system/relaystation.service` — see full docs below.

</details>

<details>
<summary><strong>🌊 DigitalOcean Droplet</strong></summary>

### Create Droplet
1. **Image:** Docker on Ubuntu (Marketplace)
2. **Size:** Basic $6/mo (1 vCPU, 1GB) — or higher for multiple streams
3. **Region:** Choose closest to your users
4. **Auth:** SSH keys (recommended)

### Production Setup
```bash
ssh root@your-droplet-ip
git clone https://github.com/ParkWardRR/relaystation.git
cd relaystation
docker compose -f docker/docker-compose.yml up -d

# Configure firewall
ufw allow OpenSSH
ufw allow 8080/tcp
ufw enable
```

**Domain + SSL:** Install Nginx + Certbot — see full docs below.

</details>

<br>

## ⚙️ Configuration

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
        "high_quality": {
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

<br>

## 🎨 Built-in Presets

| Preset | Codec | Resolution | Bitrate | Use Case |
|--------|-------|------------|---------|----------|
| **HEVC 1080p** | H.265 | 1920×1080 | 4.0 Mbps | Modern Apple devices |
| **HEVC Max Quality** | H.265 | 1920×1080 | 8.0 Mbps | Apple TV 4K, iPad Pro |
| **Legacy iPad** | H.264 | 1280×720 | 2.0 Mbps | iPad 2, iPad Mini 1-3 |
| **Modern Tablet** | H.264 | 1920×1080 | 4.0 Mbps | iPad Air, iPad Pro |
| **Apple TV 3rd Gen** | H.264 | 1280×720 | 2.5 Mbps | Apple TV 3 |
| **Mobile Optimized** | H.264 | 854×480 | 1.0 Mbps | Cellular / low bandwidth |
| **High Quality** | H.264 | 1920×1080 | 6.0 Mbps | Fast networks |
| **Passthrough** | Copy | — | — | Lowest latency |

<br>

## 🔌 API Endpoints

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

<br>

## 🧑‍💻 Development

```bash
make build        # Build Go binary
make dev          # Run in development mode
make test         # Run tests
make clean        # Clean build artifacts
make docker       # Build Docker image
make docker-up    # Start with Docker Compose
make docker-down  # Stop Docker Compose
make web-install  # Install web dependencies
make web-build    # Build web frontend
make web-dev      # Run web dev server
make all          # Full build (web + docker)
```

<br>

## 📁 Architecture

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

<br>

## 📄 License

This project is licensed under the [Blue Oak Model License 1.0.0](LICENSE).

<br>

---

<div align="center">

**[⬆ Back to Top](#relaystation)**

<br>

<sub>Built for streaming flexibility</sub>

</div>
