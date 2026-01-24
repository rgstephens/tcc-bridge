.PHONY: all build build-go build-frontend build-matter clean dev install test

# Version and build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%d")
LDFLAGS := -X github.com/gregjohnson/mitsubishi/internal/web.Version=$(VERSION) \
           -X github.com/gregjohnson/mitsubishi/internal/web.BuildDate=$(BUILD_DATE)

# Default target
all: build

# Build everything
build: build-frontend build-matter build-go

# Build Go backend
build-go:
	@echo "Building Go backend..."
	@echo "Version: $(VERSION), Build date: $(BUILD_DATE)"
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o bin/tcc-bridge ./cmd/server

# Build for Raspberry Pi (ARM64)
build-go-pi:
	@echo "Building Go backend for Raspberry Pi..."
	@echo "Version: $(VERSION), Build date: $(BUILD_DATE)"
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/tcc-bridge-arm64 ./cmd/server

# Build frontend
build-frontend:
	@echo "Building Vue frontend..."
	cd web && npm install && npm run build

# Build Matter bridge
build-matter:
	@echo "Building Matter.js bridge..."
	cd matter-bridge && npm install && npm run build

# Development mode - run all services
dev:
	@echo "Starting development servers..."
	@echo "Run these in separate terminals:"
	@echo "  make dev-matter"
	@echo "  make dev-frontend"
	@echo "  make dev-go"
	@echo ""
	@echo "Or run: make run (production build)"

# Run Go backend in dev mode
dev-go:
	go run ./cmd/server -debug

# Run frontend in dev mode
dev-frontend:
	cd web && npx vite

# Run Matter bridge in dev mode
dev-matter:
	cd matter-bridge && node dist/index.js

# Run production build (after make build)
run:
	./bin/tcc-bridge

# Install dependencies
install:
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing frontend dependencies..."
	cd web && npm install
	@echo "Installing Matter bridge dependencies..."
	cd matter-bridge && npm install

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf matter-bridge/dist/
	rm -rf matter-bridge/node_modules/
	rm -rf web/node_modules/

# Install systemd service (run as root)
install-service:
	@echo "Installing systemd service..."
	sudo cp configs/systemd/tcc-bridge.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable tcc-bridge
	@echo "Service installed. Start with: sudo systemctl start tcc-bridge"

# Uninstall systemd service
uninstall-service:
	sudo systemctl stop tcc-bridge || true
	sudo systemctl disable tcc-bridge || true
	sudo rm -f /etc/systemd/system/tcc-bridge.service
	sudo systemctl daemon-reload

# Show logs
logs:
	sudo journalctl -u tcc-bridge -f

# Format code
fmt:
	go fmt ./...
	cd web && npm run format || true

# Lint code
lint:
	go vet ./...
	cd web && npm run lint || true

# Generate go.sum
tidy:
	go mod tidy

# Help
help:
	@echo "TCC-Matter Bridge Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build           Build all components"
	@echo "  build-go        Build Go backend"
	@echo "  build-go-pi     Build Go backend for Raspberry Pi"
	@echo "  build-frontend  Build Vue frontend"
	@echo "  build-matter    Build Matter.js bridge"
	@echo "  dev             Run all services in development mode"
	@echo "  install         Install all dependencies"
	@echo "  test            Run tests"
	@echo "  clean           Clean build artifacts"
	@echo "  install-service Install systemd service"
	@echo "  logs            Show service logs"
	@echo "  help            Show this help"
