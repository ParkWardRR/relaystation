#!/bin/bash
# Deploy an IMSA Relay Droplet on DigitalOcean
# This script deploys an optimized Nginx reverse proxy droplet for IMSA streaming.
# It includes critical performance tuning for HTTP KeepAlive and high-capacity RAM buffering
# to prevent SSD IOPS exhaustion during heavy streaming.

if [ -z "$1" ]; then
    echo "Usage: ./deploy_imsa_relay.sh <DIGITAL_OCEAN_API_TOKEN>"
    exit 1
fi

DO_TOKEN=$1
DROPLET_NAME="imsa-relay-worker-$(date +%s)"
REGION="syd1" # Sydney, Australia (good for avoiding NA geo-blocks)
SIZE="s-1vcpu-512mb-10gb"
IMAGE="ubuntu-24-04-x64"

# Nginx Configuration optimized for high-throughput video streaming
read -r -d '' NGINX_CONF << 'EOF'
server {
    listen 80 default_server;
    listen [::]:80 default_server;

    resolver 8.8.8.8 8.8.4.4 valid=300s;
    resolver_timeout 5s;

    # Backend Connection Tuning
    proxy_http_version 1.1;
    proxy_set_header Connection "";

    # Super RAM Buffering
    # Allocating 16MB of RAM per active connection to aggressively pre-fetch from CloudFront
    # This prevents stuttering by keeping a massive buffer of video entirely in memory
    proxy_buffering on;
    proxy_buffers 64 256k;
    proxy_buffer_size 256k;
    proxy_busy_buffers_size 512k;
    
    # Strictly disable disk usage to prevent SSD IOPS bursting and exhaustion
    proxy_max_temp_file_size 0;

    proxy_connect_timeout 10s;
    proxy_read_timeout 30s;
    proxy_send_timeout 30s;

    location /live/imsa_international/ {
        proxy_pass https://d22t65jbw0v36j.cloudfront.net/live/imsa_international/;
        proxy_ssl_server_name on;
        proxy_set_header Host d22t65jbw0v36j.cloudfront.net;
        proxy_set_header Accept-Encoding "";

        sub_filter 'https://d22t65jbw0v36j.cloudfront.net/' 'http://$host/';
        sub_filter_once off;
        sub_filter_types application/vnd.apple.mpegurl text/plain;

        add_header Access-Control-Allow-Origin * always;
    }

    location /live/ {
        proxy_pass https://dwomb0lw8ct6d.cloudfront.net/live/;
        proxy_ssl_server_name on;
        proxy_set_header Host dwomb0lw8ct6d.cloudfront.net;
        proxy_set_header Accept-Encoding "";

        sub_filter 'https://dwomb0lw8ct6d.cloudfront.net/' 'http://$host/';
        sub_filter_once off;
        sub_filter_types application/vnd.apple.mpegurl text/plain;

        add_header Access-Control-Allow-Origin * always;
    }
}
EOF

# Escape quotes and newlines for JSON payload
ESCAPED_CONF=$(echo "$NGINX_CONF" | sed 's/"/\\"/g' | awk '{printf "%s\\n", $0}')

# Cloud-Init User Data script
USER_DATA="#cloud-config
runcmd:
  - apt-get update
  - apt-get install -y nginx curl jq at
  # Delete itself in 24 hours to save money
  - echo \"curl -s -X DELETE -H 'Content-Type: application/json' -H 'Authorization: Bearer \$DO_TOKEN' 'https://api.digitalocean.com/v2/droplets/\$(curl -s http://169.254.169.254/metadata/v1/id)'\" | at now + 24 hours
  # Deploy Tuned Nginx Config
  - echo -e \"$ESCAPED_CONF\" > /etc/nginx/sites-available/default
  - systemctl restart nginx
"

echo "Deploying Droplet: $DROPLET_NAME in $REGION..."

curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DO_TOKEN" \
  -d '{
    "name":"'"$DROPLET_NAME"'",
    "region":"'"$REGION"'",
    "size":"'"$SIZE"'",
    "image":"'"$IMAGE"'",
    "ssh_keys": [],
    "backups":false,
    "ipv6":false,
    "user_data":"'"${USER_DATA//$'\n'/\\n}"'",
    "private_networking":null,
    "volumes": null,
    "tags": ["imsa-relay"]
  }' \
  "https://api.digitalocean.com/v2/droplets" | jq .

echo ""
echo "Deployment requested. Check DigitalOcean dashboard for droplet IP."
echo "Nginx will be automatically configured with Super-Buffer tuning to prevent lag."
