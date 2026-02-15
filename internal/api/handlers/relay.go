package handlers

import (
	"log"
	"strconv"

	"github.com/ParkWardRR/relaystation/internal/relay"
	"github.com/ParkWardRR/relaystation/internal/scanner"
	"github.com/gofiber/fiber/v2"
)

// RelayHandler provides HTTP handlers for the relay control API and dashboard.
// It exposes status, source switching, stream scanning, and serves the
// embedded web dashboard.
type RelayHandler struct {
	relay   *relay.Relay
	scanner *scanner.Scanner
}

// NewRelayHandler creates a new handler bound to the given relay instance.
func NewRelayHandler(r *relay.Relay) *RelayHandler {
	return &RelayHandler{
		relay:   r,
		scanner: scanner.NewScanner(),
	}
}

// GetStatus returns the current relay status as JSON (GET /api/relay/status).
func (h *RelayHandler) GetStatus(c *fiber.Ctx) error {
	return c.JSON(h.relay.GetStatus())
}

// SwitchSource switches the relay to a different source by index
// (POST /api/relay/switch/:idx). The switch is instant — the old FFmpeg
// process is killed and a new one is started within ~200ms.
func (h *RelayHandler) SwitchSource(c *fiber.Ctx) error {
	idxStr := c.Params("idx")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid source index"})
	}

	log.Printf("[RelayHandler] Switching to source %d", idx)
	if err := h.relay.SwitchSource(idx); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"ok": true, "switched_to": idx})
}

// ScanSources scans one or more web pages for stream URLs
// (POST /api/relay/scan). Expects JSON body: {"urls": ["https://..."]}.
func (h *RelayHandler) ScanSources(c *fiber.Ctx) error {
	type ScanRequest struct {
		URLs []string `json:"urls"`
	}

	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	if len(req.URLs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "urls array is required"})
	}

	log.Printf("[RelayHandler] Scanning %d pages for streams...", len(req.URLs))
	results := h.scanner.ScanPages(req.URLs)

	return c.JSON(fiber.Map{"results": results})
}

// Dashboard serves the relay control dashboard HTML
func (h *RelayHandler) Dashboard(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>RelayStation — Stream Control</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#0a0a0f;--surface:#12121a;--surface2:#1a1a26;--border:#2a2a3a;
  --text:#e4e4ef;--text2:#8888a0;--accent:#6c5ce7;--accent2:#a29bfe;
  --green:#00b894;--red:#ff6b6b;--yellow:#ffd93d;--orange:#fd9644;
}
body{font-family:'Inter',system-ui,sans-serif;background:var(--bg);color:var(--text);min-height:100vh}
.container{max-width:900px;margin:0 auto;padding:24px 16px}
header{display:flex;align-items:center;justify-content:space-between;margin-bottom:32px}
h1{font-size:22px;font-weight:700;background:linear-gradient(135deg,var(--accent),var(--accent2));-webkit-background-clip:text;-webkit-text-fill-color:transparent}
.status-badge{display:flex;align-items:center;gap:8px;font-size:13px;color:var(--text2);background:var(--surface);padding:6px 14px;border-radius:20px;border:1px solid var(--border)}
.status-dot{width:8px;height:8px;border-radius:50%;animation:pulse 2s infinite}
.status-dot.live{background:var(--green)}
.status-dot.off{background:var(--red);animation:none}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}
.info-bar{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin-bottom:28px}
.info-card{background:var(--surface);border:1px solid var(--border);border-radius:12px;padding:14px 16px}
.info-card .label{font-size:11px;text-transform:uppercase;letter-spacing:.8px;color:var(--text2);margin-bottom:4px}
.info-card .value{font-size:18px;font-weight:600}
.section-title{font-size:14px;font-weight:600;color:var(--text2);text-transform:uppercase;letter-spacing:1px;margin-bottom:14px}
.sources{display:flex;flex-direction:column;gap:10px}
.source-card{background:var(--surface);border:1px solid var(--border);border-radius:12px;padding:16px 18px;display:flex;align-items:center;gap:16px;cursor:pointer;transition:all .15s ease;position:relative;overflow:hidden}
.source-card:hover{border-color:var(--accent);transform:translateY(-1px);box-shadow:0 4px 20px rgba(108,92,231,.1)}
.source-card.active{border-color:var(--accent);background:linear-gradient(135deg,rgba(108,92,231,.08),rgba(162,155,254,.04))}
.source-card.active::before{content:'';position:absolute;left:0;top:0;bottom:0;width:3px;background:linear-gradient(to bottom,var(--accent),var(--accent2))}
.source-card.switching{opacity:.6;pointer-events:none}
.source-idx{width:28px;height:28px;border-radius:8px;background:var(--surface2);display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:600;color:var(--text2);flex-shrink:0}
.source-card.active .source-idx{background:var(--accent);color:#fff}
.source-info{flex:1;min-width:0}
.source-label{font-size:15px;font-weight:600;margin-bottom:3px;display:flex;align-items:center;gap:8px}
.source-meta{font-size:12px;color:var(--text2);display:flex;gap:12px;flex-wrap:wrap}
.source-meta span{display:flex;align-items:center;gap:4px}
.health-dot{width:7px;height:7px;border-radius:50%;flex-shrink:0}
.health-dot.up{background:var(--green)}
.health-dot.down{background:var(--red)}
.health-dot.unknown{background:var(--text2)}
.badge{font-size:10px;padding:2px 8px;border-radius:6px;font-weight:600;text-transform:uppercase;letter-spacing:.5px}
.badge.live{background:rgba(0,184,148,.15);color:var(--green)}
.badge.active{background:rgba(108,92,231,.15);color:var(--accent2)}
.source-action{flex-shrink:0}
.switch-btn{background:var(--surface2);border:1px solid var(--border);color:var(--text2);padding:6px 14px;border-radius:8px;font-size:12px;font-weight:600;cursor:pointer;transition:all .15s}
.switch-btn:hover{background:var(--accent);color:#fff;border-color:var(--accent)}
.source-card.active .switch-btn{opacity:.4;pointer-events:none}
.vlc-bar{margin-top:28px;background:var(--surface);border:1px solid var(--border);border-radius:12px;padding:16px 18px}
.vlc-bar .label{font-size:11px;text-transform:uppercase;letter-spacing:.8px;color:var(--text2);margin-bottom:6px}
.vlc-url{font-family:'SF Mono',Monaco,monospace;font-size:13px;color:var(--accent2);word-break:break-all;user-select:all;cursor:text}
.toast{position:fixed;bottom:24px;right:24px;background:var(--accent);color:#fff;padding:10px 20px;border-radius:10px;font-size:13px;font-weight:500;opacity:0;transform:translateY(10px);transition:all .3s;pointer-events:none;z-index:100}
.toast.show{opacity:1;transform:translateY(0)}
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>⚡ RelayStation</h1>
    <div class="status-badge">
      <div class="status-dot" id="statusDot"></div>
      <span id="statusText">Connecting...</span>
    </div>
  </header>

  <div class="info-bar">
    <div class="info-card">
      <div class="label">Active Source</div>
      <div class="value" id="activeLabel">—</div>
    </div>
    <div class="info-card">
      <div class="label">Bandwidth</div>
      <div class="value" id="activeBW">—</div>
    </div>
    <div class="info-card">
      <div class="label">Resolution</div>
      <div class="value" id="activeRes">—</div>
    </div>
    <div class="info-card">
      <div class="label">Uptime</div>
      <div class="value" id="uptime">—</div>
    </div>
    <div class="info-card">
      <div class="label">Restarts</div>
      <div class="value" id="restarts">0</div>
    </div>
    <div class="info-card">
      <div class="label">Failovers</div>
      <div class="value" id="failovers">0</div>
    </div>
  </div>

  <div class="section-title">Sources</div>
  <div class="sources" id="sourcesList"></div>

  <div class="vlc-bar">
    <div class="label">VLC Network Stream URL</div>
    <div class="vlc-url" id="vlcUrl">—</div>
  </div>
</div>

<div class="toast" id="toast"></div>

<script>
const API = '/api/relay';
let currentStatus = null;
let switching = false;

function formatBW(bw) {
  if (!bw) return '—';
  if (bw >= 1000000) return (bw/1000000).toFixed(1) + ' Mbps';
  if (bw >= 1000) return (bw/1000).toFixed(0) + ' Kbps';
  return bw + ' bps';
}

function showToast(msg) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.classList.add('show');
  setTimeout(() => t.classList.remove('show'), 2500);
}

async function fetchStatus() {
  try {
    const r = await fetch(API + '/status');
    currentStatus = await r.json();
    render(currentStatus);
  } catch(e) {
    document.getElementById('statusDot').className = 'status-dot off';
    document.getElementById('statusText').textContent = 'Offline';
  }
}

function render(s) {
  const dot = document.getElementById('statusDot');
  const text = document.getElementById('statusText');
  
  if (s.running) {
    dot.className = 'status-dot live';
    text.textContent = 'Live';
  } else {
    dot.className = 'status-dot off';
    text.textContent = 'Stopped';
  }

  const active = s.active_source;
  document.getElementById('activeLabel').textContent = active ? active.label : '—';
  document.getElementById('activeBW').textContent = active ? formatBW(active.max_bandwidth) : '—';
  document.getElementById('activeRes').textContent = active ? (active.max_resolution || '—') : '—';
  document.getElementById('uptime').textContent = s.uptime || '—';
  document.getElementById('restarts').textContent = s.restart_count || 0;
  document.getElementById('failovers').textContent = s.failover_count || 0;

  // VLC URL
  const host = window.location.host;
  document.getElementById('vlcUrl').textContent = 'http://' + host + '/hls/relay/nascar/stream.m3u8';

  // Sources list
  const list = document.getElementById('sourcesList');
  list.innerHTML = '';

  (s.all_sources || []).forEach((src, i) => {
    const isActive = i === s.active_idx && s.running;
    const card = document.createElement('div');
    card.className = 'source-card' + (isActive ? ' active' : '') + (switching ? ' switching' : '');

    const healthClass = src.healthy ? 'up' : (src.probed ? 'down' : 'unknown');

    card.innerHTML = 
      '<div class="source-idx">' + i + '</div>' +
      '<div class="source-info">' +
        '<div class="source-label">' +
          '<div class="health-dot ' + healthClass + '"></div>' +
          src.label +
          (isActive ? ' <span class="badge active">RELAYING</span>' : '') +
          (src.healthy && !isActive ? ' <span class="badge live">LIVE</span>' : '') +
        '</div>' +
        '<div class="source-meta">' +
          '<span>📡 ' + formatBW(src.max_bandwidth) + '</span>' +
          (src.max_resolution ? '<span>📺 ' + src.max_resolution + '</span>' : '') +
        '</div>' +
      '</div>' +
      '<div class="source-action">' +
        '<button class="switch-btn" onclick="switchTo(' + i + ')">' +
          (isActive ? 'Active' : 'Switch') +
        '</button>' +
      '</div>';

    list.appendChild(card);
  });
}

async function switchTo(idx) {
  if (switching) return;
  switching = true;
  render(currentStatus);
  showToast('Switching...');

  try {
    const r = await fetch(API + '/switch/' + idx, { method: 'POST' });
    const data = await r.json();
    if (data.ok) {
      showToast('Switched to source ' + idx);
    } else {
      showToast('Error: ' + (data.error || 'unknown'));
    }
  } catch(e) {
    showToast('Switch failed: ' + e.message);
  }

  switching = false;
  // Rapid poll after switch
  setTimeout(fetchStatus, 300);
  setTimeout(fetchStatus, 1000);
  setTimeout(fetchStatus, 2000);
}

// Poll every 2 seconds
fetchStatus();
setInterval(fetchStatus, 2000);
</script>
</body>
</html>`
