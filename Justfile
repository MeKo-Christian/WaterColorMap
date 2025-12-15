# WaterColorMap Justfile
# Task orchestration for development and building

# Default recipe - show available commands
default:
    @just --list

# Install dependencies
deps:
    go mod download
    go mod tidy

# Build the application
build:
    CGO_ENABLED=1 go build -o bin/watercolormap ./cmd/watercolormap

# Build with version information
build-release version:
    CGO_ENABLED=1 go build -ldflags "-X github.com/MeKo-Tech/watercolormap/internal/cmd.version={{version}} -X github.com/MeKo-Tech/watercolormap/internal/cmd.commit=$(git rev-parse HEAD) -X github.com/MeKo-Tech/watercolormap/internal/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/watercolormap ./cmd/watercolormap

# Run the application
run *args:
    go run ./cmd/watercolormap {{args}}

# Run tests
test:
    go test ./... -v

# Run tests with coverage
test-coverage:
    go test ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
    go fmt ./...

# Lint code
lint:
    golangci-lint run

# Lint and fix issues
lint-fix:
    golangci-lint run --fix

# Clean build artifacts
clean:
    rm -rf bin/
    rm -f coverage.out coverage.html

# Clean generated tiles
clean-tiles:
    rm -rf tiles/*.png

# Install the binary to $GOPATH/bin
install:
    go install ./cmd/watercolormap

# Generate a single tile (example for Hanover)
generate-tile zoom="13" x="4299" y="2740":
    go run ./cmd/watercolormap generate --zoom {{zoom}} --x {{x}} --y {{y}}

# Setup development environment
setup:
    @echo "Setting up development environment..."
    go mod download
    go mod tidy
    mkdir -p bin tiles assets/textures
    @echo "Setup complete!"

# Watch for changes and rebuild (requires entr)
watch:
    find . -name '*.go' | entr -r just run

# Check for security vulnerabilities
security:
    gosec ./...

# Run all quality checks
check: fmt lint test
    @echo "All checks passed!"

# Development setup - initialize everything needed
dev-init: setup deps
    @echo "Development environment ready!"
    @echo "Run 'just run' to start the application"

# Install system dependencies (Ubuntu/Debian)
install-deps:
    @echo "Installing system dependencies..."
    sudo apt-get update
    sudo apt-get install -y \
        build-essential \
        pkg-config \
        libmapnik-dev \
        mapnik-utils \
        python3-mapnik

# Verify Mapnik installation
verify-mapnik:
    @echo "Verifying Mapnik installation..."
    @mapnik-config --version || (echo "ERROR: mapnik-config not found" && exit 1)
    @pkg-config --modversion mapnik || (echo "ERROR: pkg-config cannot find mapnik" && exit 1)
    @echo "Mapnik is properly installed!"

# Build Docker image
docker-build:
    @echo "Building Docker image..."
    docker build -t watercolormap:latest .

# Run Docker container
docker-run:
    @echo "Running Docker container..."
    docker-compose run --rm watercolormap

# Start development Docker container
docker-dev:
    @echo "Starting development container..."
    docker-compose run --rm dev bash

# Generate a test tile (example)
generate-test-tile:
    @echo "Generating test tile..."
    ./bin/watercolormap generate --tile z13_x4297_y2754
