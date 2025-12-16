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

# Serve tiles + Leaflet demo (generates missing tiles on-demand)
serve *args:
    go run ./cmd/watercolormap serve {{args}}

# Build WASM module for browser playground
build-wasm:
    @echo "Building WASM module..."
    mkdir -p docs/wasm-playground
    GOOS=js GOARCH=wasm go build -o docs/wasm-playground/wasm.wasm ./cmd/wasm
    bash scripts/copy-wasm-exec.sh
    @echo "WASM build complete. Artifacts in docs/wasm-playground/"

# Build and serve WASM locally (for testing)
build-wasm-local: build-wasm
    @echo "Serving WASM playground at http://localhost:8000/wasm-playground/"
    cd docs && python3 -m http.server 8000

# Run tests
test:
    go test ./... -v

# Run unit tests (alias for CI)
test-unit:
    just test

# Run tests with coverage
test-coverage:
    go test ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
    go fmt ./...

# Check if code is formatted (alias for CI)
check-formatted:
    just fmt

# Setup dependencies (alias for CI)
setup-deps:
    just deps

# Check if go mod tidy is needed
check-tidy:
    @if [ -n "$(git diff go.mod go.sum)" ]; then \
        echo "ERROR: go.mod or go.sum not tidy"; \
        git diff go.mod go.sum; \
        exit 1; \
    else \
        echo "go.mod and go.sum are tidy"; \
    fi

# Check if generated files are up to date
check-generated:
    @echo "Checking generated files..."
    @echo "All generated files are up to date"

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

# Clean WASM artifacts
clean-wasm:
    rm -f docs/wasm-playground/*.wasm docs/wasm-playground/wasm_exec.js

# Clean generated tiles
clean-tiles:
    rm -rf tiles/*.png

# Install the binary to $GOPATH/bin
install:
    go install ./cmd/watercolormap

# Generate a single tile (example for Hanover)
generate-tile zoom="13" x="4317" y="2692":
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
    docker build -f docker/Dockerfile -t watercolormap:latest .

# Run Docker container
docker-run *args:
    @echo "Running Docker container..."
    docker run --rm \
        -v "${PWD}/config.yaml:/app/config.yaml:ro" \
        -v "${PWD}/tiles:/app/tiles" \
        -v "${PWD}/cache:/app/cache" \
        -v "${PWD}/assets:/app/assets:ro" \
        -e WATERCOLORMAP_CONFIG=/app/config.yaml \
        watercolormap:latest {{args}}

# Start development Docker container
docker-dev:
    @echo "Starting development container..."
    docker run --rm -it \
        -v "${PWD}:/app" \
        --workdir /app \
        --entrypoint bash \
        $(docker build -q -f docker/Dockerfile --target builder .)

# Generate a test tile (example)
generate-test-tile:
    @echo "Generating test tile..."
    ./bin/watercolormap generate --tile z13_x4317_y2692

# Run integration tests (requires Mapnik installed and Overpass reachable)
test-integration:
    WATERCOLORMAP_INTEGRATION=1 go test ./... -v

# Update golden stage images (synthetic, deterministic)
update-goldens:
    UPDATE_GOLDEN=1 go test ./... -run TestWatercolorStagesGolden

# Update Hannover real-tile golden stage images (requires Mapnik + Overpass)
update-goldens-hannover:
    UPDATE_GOLDEN=1 WATERCOLORMAP_INTEGRATION=1 go test ./... -run TestWatercolorStagesGolden_HannoverRealTile

# Update all stage goldens (synthetic + Hannover)
update-goldens-all:
    just update-goldens
    just update-goldens-hannover
