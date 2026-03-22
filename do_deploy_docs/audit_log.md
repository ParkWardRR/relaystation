# DigitalOcean IMSA Relay Audit Log

**Date:** March 21-22, 2026
**Target Event:** IMSA 12 Hours Live Stream Proxying

## 1. Initial Deployment & Stream Extraction
- **Action:** Extracted raw m3u8 playlists for IMSA International (`https://d22t65jbw0v36j.cloudfront.net`) and 14 In-Car Cameras.
- **Action:** Deployed a `$4/mo` 1vCPU / 512MB basic droplet in the `syd1` (Sydney, AU) region using the DigitalOcean API.
- **Action:** Configured Nginx as a standard reverse proxy (HTTP/1.0 upstream) to bypass geo-blocking.
- **Result:** Successfully bypassed CloudFront 403 blocks. VLC playback confirmed at `http://209.38.25.42/live/imsa_international/master.m3u8`.

## 2. Codebase Integration
- **Action:** Retargeted the `ParkWardRR/relaystation` codebase (originally configured for NASCAR) to point to the new DO droplet.
- **Action:** Updated Go logic (`main.go`, `relay.go`) to spin up `imsaRelay` failover sources targeting the Dropet IPs instead of DD12 CDNs.
- **Action:** Shifted the `commercial_detector` machine-learning listener to monitor the `IMSA-International` relay stream.
- **Action:** Updated tests, configuration (`streams.json`), frontend code, and `README.md` to cleanly transition to the IMSA use cases.

## 3. Incident: Stream Latency & IOPS Exhaustion
- **Symptom:** After ~5 hours of flawless streaming, the droplet became extremely laggy, resulting in severe VLC buffering.
- **Investigation:** System audit (`top`, `free`, `nginx logs`) revealed:
  - DO Bandwidth Quota was healthy (only `21 GB` out of `500 GB` consumed).
  - CPU usage was idle.
  - **Root Cause:** Nginx `proxy_buffering` was defaulting to a tiny 128KB memory buffer. Because IMSA 1080p chunks are ~3.2MB each, Nginx spooled the overflow to temporary files on the SSD. Continual 3.2MB reads/writes every 3 seconds exhausted the Droplet's SSD Burst IOPS bucket, causing extreme disk I/O wait.

## 4. Remediation & Hotfix
- **Action:** Engineered an Nginx tuning profile to use **Super-RAM Buffering**. 
  - `proxy_buffers 64 256k;` (Allocated up to 16MB of RAM per proxy connection).
  - `proxy_max_temp_file_size 0;` (Strictly disabled writing anything to disk).
  - `proxy_http_version 1.1` and `proxy_set_header Connection ""` (Enabled upstream HTTP KeepAlive to stop recreating TLS sessions for every chunk).
- **Result:** Hotfix resolved all latency instantly. The stream became buttery smooth as all proxy processing utilized high-speed RAM.

## 5. Automation & Cleanup
- **Action:** Encapsulated the Nginx tuning + Droplet initialization into a reusable bash script: `scripts/deploy_imsa_relay.sh`.
- **Action:** Updated `README.md` to mandate the script for future relays to permanently mitigate the IOPS bottleneck.
- **Action:** Evaluated the system limits, concluding the $4/mo DO Droplet can comfortably support 150-200 concurrent viewers on throughput alone (bottleneck is strictly the 500GB monthly quota).
- **Action:** The live Droplet (`ID: 559929852`) was successfully deleted via DO API requests at 08:03 AM to halt active billing.
