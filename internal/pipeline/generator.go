package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/MeKo-Tech/watercolormap/internal/composite"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/renderer"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/MeKo-Tech/watercolormap/internal/watercolor"
	"log/slog"
)

// DataSource fetches OSM features for a tile coordinate.
type DataSource interface {
	FetchTileData(context.Context, types.TileCoordinate) (*types.TileData, error)
}

// Generator wires datasource, rendering, watercolor, and compositing into a single step.
type Generator struct {
	ds         DataSource
	stylesDir  string
	outputDir  string
	textures   map[geojson.LayerType]image.Image
	tileSize   int
	seed       int64
	keepLayers bool
	logger     *slog.Logger
}

// NewGenerator loads textures and prepares a generator.
func NewGenerator(ds DataSource, stylesDir, texturesDir, outputDir string, tileSize int, seed int64, keepLayers bool, logger *slog.Logger) (*Generator, error) {
	if tileSize <= 0 {
		return nil, fmt.Errorf("tile size must be positive")
	}

	textures, err := texture.LoadDefaultTextures(texturesDir)
	if err != nil {
		return nil, err
	}

	return &Generator{
		ds:         ds,
		stylesDir:  stylesDir,
		outputDir:  outputDir,
		textures:   textures,
		tileSize:   tileSize,
		seed:       seed,
		keepLayers: keepLayers,
		logger:     logger,
	}, nil
}

// Generate renders, paints, composites, and writes the final tile PNG.
// Returns the final tile path and (optionally) the layer directory when kept.
func (g *Generator) Generate(ctx context.Context, coords tile.Coords, force bool) (string, string, error) {
	finalPath := filepath.Join(g.outputDir, coords.Path("png"))
	if !force {
		if _, err := os.Stat(finalPath); err == nil {
			g.log().Info("Tile already exists; skipping", "coords", coords.String(), "path", finalPath)
			return finalPath, "", nil
		}
	}

	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create output dir: %w", err)
	}

	g.log().Info("Fetching tile data", "coords", coords.String())
	data, err := g.ds.FetchTileData(ctx, types.TileCoordinate{
		Zoom: int(coords.Z),
		X:    int(coords.X),
		Y:    int(coords.Y),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch tile data: %w", err)
	}

	layerDir, err := os.MkdirTemp("", "watercolormap-layers-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp layer dir: %w", err)
	}
	layerDirReturn := ""
	if g.keepLayers {
		layerDirReturn = layerDir
		g.log().Info("Keeping rendered layer PNGs", "coords", coords.String(), "dir", layerDir)
	} else {
		defer os.RemoveAll(layerDir) // nolint:errcheck // best effort cleanup
	}

	g.log().Info("Rendering layers", "coords", coords.String())
	mpRenderer, err := renderer.NewMultiPassRenderer(g.stylesDir, layerDir, g.tileSize)
	if err != nil {
		return "", "", fmt.Errorf("failed to create multipass renderer: %w", err)
	}
	defer mpRenderer.Close() // nolint:errcheck

	renderResult, err := mpRenderer.RenderTile(coords, data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render layers: %w", err)
	}

	params := watercolor.DefaultParams(g.tileSize, g.seed, g.textures)
	params.OffsetX = int(coords.X) * g.tileSize
	params.OffsetY = int(coords.Y) * g.tileSize

	painted := make(map[geojson.LayerType]image.Image)

	for layer, res := range renderResult.Layers {
		if res == nil || res.OutputPath == "" {
			g.log().Debug("Skipping empty layer", "layer", layer, "coords", coords.String())
			continue // layer had no features
		}
		if res.Error != nil {
			return "", "", fmt.Errorf("failed to render layer %s: %w", layer, res.Error)
		}

		g.log().Debug("Painting layer", "layer", layer, "coords", coords.String())
		img, err := readPNG(res.OutputPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read layer %s: %w", layer, err)
		}

		paintedLayer, err := watercolor.PaintLayer(img, layer, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint layer %s: %w", layer, err)
		}

		painted[layer] = paintedLayer
	}

	composited, err := composite.CompositeLayers(painted, nil, g.tileSize)
	if err != nil {
		return "", "", fmt.Errorf("failed to composite layers: %w", err)
	}

	g.log().Info("Writing final tile", "coords", coords.String(), "path", finalPath)
	outFile, err := os.Create(finalPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create tile file: %w", err)
	}
	defer outFile.Close() // nolint:errcheck

	if err := png.Encode(outFile, composited); err != nil {
		return "", "", fmt.Errorf("failed to encode final tile: %w", err)
	}

	return finalPath, layerDirReturn, nil
}

func readPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := png.Decode(file)
	return img, err
}

func (g *Generator) log() *slog.Logger {
	if g.logger != nil {
		return g.logger
	}
	return slog.Default()
}
