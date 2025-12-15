# WaterColorMap

A watercolor-styled map tile generator that creates beautiful, artistic map tiles from OpenStreetMap data.

## Overview

WaterColorMap fetches OpenStreetMap data for specific tiles, applies watercolor rendering techniques, and outputs styled map tiles suitable for web mapping applications.

## Features

- 🎨 Watercolor-style rendering of map features
- 🗺️ Tile-based coordinate system (z/x/y)
- 📦 Multiple OSM data sources (Overpass API, Protomaps, etc.)
- ⚡ Efficient tile caching and regeneration control
- 🛠️ CLI-based workflow with Cobra/Viper
- 🎯 Task orchestration with Just

## Prerequisites

- Go 1.25 or higher
- [Just](https://github.com/casey/just) command runner (optional but recommended)
- Git
- **Mapnik 3.1+** and development libraries (libmapnik-dev)
- pkg-config
- C++ compiler (g++ or clang)

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/MeKo-Tech/watercolormap.git
cd watercolormap
```

### 2. Install System Dependencies

On Ubuntu/Debian:
```bash
sudo apt update
sudo apt install -y libmapnik-dev mapnik-utils build-essential pkg-config
```

Or use the provided Justfile:
```bash
just install-deps
```

### 3. Setup Go Environment

Using Just (recommended):
```bash
just build
```

Or manually:
```bash
go mod download
go mod tidy
mkdir -p tiles cache testdata/output
CGO_ENABLED=1 go build -o watercolormap ./cmd/watercolormap
```

### 4. Configure

Copy the example configuration:
```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` to customize settings.

### 5. Run

```bash
./watercolormap --help
```

## Docker Support

Build and run using Docker:

```bash
# Build image
just docker-build

# Run container
docker-compose run watercolormap --help

# Development container with shell access
just docker-dev
```

## Usage

### Generate a Tile

```bash
# Generate a single tile
watercolormap generate --zoom 13 --x 4299 --y 2740

# Force regeneration
watercolormap generate -z 13 -x 4299 -y 2740 --force

# Using Just
just generate-tile 13 4299 2740
```

### Available Commands

```bash
watercolormap --help          # Show help
watercolormap version         # Show version information
watercolormap generate        # Generate map tiles
```

### Configuration

Configuration can be provided via:
1. Configuration file (`config.yaml`)
2. Environment variables (prefix: `WATERCOLORMAP_`)
3. Command-line flags

Example:
```bash
# Via config file
watercolormap generate --config ./config.yaml

# Via environment variable
export WATERCOLORMAP_OUTPUT_DIR=./my-tiles
watercolormap generate

# Via flag
watercolormap generate --output-dir ./my-tiles
```

## Development

### Available Just Commands

```bash
just                    # Show all available commands
just build              # Build the application
just test               # Run tests
just test-coverage      # Run tests with coverage
just fmt                # Format code
just lint               # Run linter
just check              # Run all quality checks (fmt + lint + test)
just clean              # Clean build artifacts
just docker-build       # Build Docker image
just docker-run         # Run in Docker
just docker-dev         # Start development container
```

### Project Structure

```
.
├── cmd/
│   └── watercolormap/        # CLI entry point
├── internal/
│   ├── cmd/                  # Cobra commands
│   ├── types/                # Core types (TileCoordinate, Feature, etc.)
│   ├── datasource/           # OSM data fetching (Overpass API)
│   └── renderer/             # Mapnik rendering wrapper
├── assets/
│   ├── textures/             # Watercolor textures (future)
│   └── styles/               # Mapnik XML styles
├── docs/                     # Documentation
├── tiles/                    # Generated tiles (gitignored)
├── cache/                    # Data cache (gitignored)
├── testdata/                 # Test data and outputs
├── config.example.yaml       # Example configuration
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml       # Container orchestration
├── Justfile                 # Development automation
└── go.mod                   # Go module definition
```

### Testing

```bash
just test              # Run all tests
just test-coverage     # Generate coverage report with HTML output
```

Unit tests are in `*_test.go` files alongside source code. Integration tests require Mapnik and may fetch data from Overpass API.

Run only short tests (skips integration):
```bash
go test -short ./...
```

### Code Quality

```bash
just fmt              # Format code
just lint             # Run linters
just check            # Run all quality checks (fmt + lint + test)
```

## Configuration Reference

See [config.example.yaml](config.example.yaml) for a complete configuration reference with all available options.

### Key Configuration Sections

- **data-source**: OSM data source selection
- **output-dir**: Where to save generated tiles
- **overpass**: Overpass API settings (endpoint, rate limiting, retry logic)
- **tile**: Tile generation settings (size, format, DPI)
- **rendering**: Rendering configuration (textures, layers, colors)
- **test-area**: Hanover test area configuration

## Roadmap

See [PLAN.md](PLAN.md) for the detailed development plan.

### Phase 1: Data Preparation and Tool Setup ✅
- ✅ Go project structure
- ✅ CLI framework (Cobra/Viper)
- ✅ Configuration system (YAML + env vars)
- ✅ Tile coordinate system and storage
- ✅ OSM data fetching (Overpass API)
- ✅ Map rendering tools (Mapnik integration)
- ✅ Docker support
- ⬜ Texture preparation (next step)

### Future Phases
- Phase 2: Basic Tile Rendering
- Phase 3: Watercolor Effect Application
- Phase 4: Area Rendering and Optimization
- Phase 5: Deployment and Scaling

## Contributing

Contributions are welcome! Please ensure:
- Code passes all tests (`just test`)
- Code is formatted (`just fmt`)
- Linting passes (`just lint`)
- All quality checks pass (`just check`)

## License

[Add your license here]

## Acknowledgments

- OpenStreetMap contributors for map data
- Stamen Design for watercolor map inspiration
