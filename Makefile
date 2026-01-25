.PHONY: build dev test clean docker docker-dev

# Build the Go binary
build:
	go build -o bin/relaystation ./cmd/relaystation

# Run in development mode
dev:
	go run ./cmd/relaystation

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/ tmp/ web/build/ web/.svelte-kit/

# Build Docker image
docker:
	docker build -t relaystation:latest -f docker/Dockerfile .

# Run with Docker Compose
docker-up:
	docker compose -f docker/docker-compose.yml up -d

# Stop Docker Compose
docker-down:
	docker compose -f docker/docker-compose.yml down

# Development with Docker Compose
docker-dev:
	docker compose -f docker/docker-compose.dev.yml up

# Install web dependencies
web-install:
	cd web && npm install

# Build web frontend
web-build:
	cd web && npm run build

# Run web dev server
web-dev:
	cd web && npm run dev

# Full build (web + docker)
all: web-install web-build docker
