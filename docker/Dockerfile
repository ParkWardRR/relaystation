# Stage 1: Build Go backend
FROM golang:1.22-alpine AS go-builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod file and source
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/

# Download dependencies
RUN go mod tidy && go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o relaystation ./cmd/relaystation

# Stage 2: Build SvelteKit frontend
FROM node:20-alpine AS web-builder

WORKDIR /app/web

# Copy package files and install
COPY web/package*.json ./
RUN npm install

# Copy source
COPY web/ ./

# Build
RUN npm run build

# Stage 3: Final image
FROM alpine:3.19

# Install FFmpeg and other dependencies
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    curl

WORKDIR /app

# Copy binary from Go builder
COPY --from=go-builder /app/relaystation .

# Copy static files from web builder
COPY --from=web-builder /app/web/build ./web/build

# Copy default config
COPY configs/streams.json /etc/relaystation/streams.json

# Create directories
RUN mkdir -p /var/www/hls /var/log/relaystation

# Environment
ENV RELAYSTATION_CONFIG=/etc/relaystation/streams.json
ENV RELAYSTATION_OUTPUT=/var/www/hls
ENV RELAYSTATION_STATIC=/app/web/build
ENV RELAYSTATION_ADDR=:8080

# Expose port
EXPOSE 8080

# Volumes for persistence
VOLUME ["/etc/relaystation", "/var/www/hls"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

# Run
ENTRYPOINT ["./relaystation"]
