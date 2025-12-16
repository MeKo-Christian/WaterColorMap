package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/MeKo-Tech/watercolormap/internal/composite"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/renderer"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/MeKo-Tech/watercolormap/internal/watercolor"
)

// DataSource fetches OSM features for a tile coordinate.
type DataSource interface {
	FetchTileData(context.Context, types.TileCoordinate) (*types.TileData, error)
}

// Generator wires datasource, rendering, watercolor, and compositing into a single step.
type Generator struct {
	ds         DataSource
	textures   map[geojson.LayerType]image.Image
	logger     *slog.Logger
	stylesDir  string
	outputDir  string
	tileSize   int
	seed       int64
	keepLayers bool
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
	raw := make(map[geojson.LayerType]image.Image)

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

		raw[layer] = img
	}

	// --- Phase 3 (revised): cross-layer mask construction ---
	// water = sea (same in our data)
	waterImg := raw[geojson.LayerWater]
	roadsImg := raw[geojson.LayerRoads]
	highwaysImg := raw[geojson.LayerHighways]

	baseBounds := image.Rect(0, 0, g.tileSize, g.tileSize)
	waterMask := mask.NewEmptyMask(baseBounds)
	roadsMask := mask.NewEmptyMask(baseBounds)
	if waterImg != nil {
		waterMask = mask.ExtractAlphaMask(waterImg)
	}
	if roadsImg != nil {
		roadsMask = mask.ExtractAlphaMask(roadsImg)
	}

	nonLandBase := mask.MaxMask(waterMask, roadsMask)

	// Paint water from its own alpha mask (not the combined non-land mask).
	if waterImg != nil {
		waterPainted, err := watercolor.PaintLayer(waterImg, geojson.LayerWater, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint water: %w", err)
		}
		painted[geojson.LayerWater] = waterPainted
	}

	// Build nonLand mask using the standard mask pipeline (blur/noise/threshold/AA), then invert for land.
	// We reuse the same parameters as other layers (but operate on the combined mask).
	finalNonLandMask, err := func() (*image.Gray, error) {
		blurred := mask.GaussianBlur(nonLandBase, params.BlurSigma)
		noise := mask.GeneratePerlinNoiseWithOffset(params.TileSize, params.TileSize, params.NoiseScale, params.Seed, params.OffsetX, params.OffsetY)
		noisy := blurred
		if params.NoiseStrength != 0 {
			noisy = mask.ApplyNoiseToMask(blurred, noise, params.NoiseStrength)
		}
		thresholded := mask.ApplyThreshold(noisy, params.Threshold)
		finalMask := thresholded
		if params.AntialiasSigma > 0 {
			finalMask = mask.AntialiasEdges(thresholded, params.AntialiasSigma)
		}
		return finalMask, nil
	}()
	if err != nil {
		return "", "", fmt.Errorf("failed to process non-land mask: %w", err)
	}
	landMask := mask.InvertMask(finalNonLandMask)

	// Paint land directly from derived land mask.
	paintedLand, err := watercolor.PaintLayerFromFinalMask(landMask, geojson.LayerLand, params)
	if err != nil {
		return "", "", fmt.Errorf("failed to paint land from derived mask: %w", err)
	}
	painted[geojson.LayerLand] = paintedLand

	// Paint roads from their own alpha mask.
	// NOTE: Roads are also part of the derived non-land union mask, so they carve holes
	// into land. Painting roads fills those holes with the intended style (instead of
	// leaving paper showing through).
	if roadsImg != nil {
		roadsPainted, err := watercolor.PaintLayer(roadsImg, geojson.LayerRoads, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint roads: %w", err)
		}
		painted[geojson.LayerRoads] = roadsPainted
	}

	// Paint highways/major roads on top.
	if highwaysImg != nil {
		highwaysPainted, err := watercolor.PaintLayer(highwaysImg, geojson.LayerHighways, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint highways: %w", err)
		}
		painted[geojson.LayerHighways] = highwaysPainted
	}

	// Constrain parks/civic to land, then paint.
	if parksImg := raw[geojson.LayerParks]; parksImg != nil {
		parksMask := mask.MinMask(mask.ExtractAlphaMask(parksImg), landMask)
		parksPainted, err := watercolor.PaintLayerFromMask(parksMask, geojson.LayerParks, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint parks constrained to land: %w", err)
		}
		painted[geojson.LayerParks] = parksPainted
	}
	if civicImg := raw[geojson.LayerCivic]; civicImg != nil {
		civicMask := mask.MinMask(mask.ExtractAlphaMask(civicImg), landMask)
		civicPainted, err := watercolor.PaintLayerFromMask(civicMask, geojson.LayerCivic, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint civic constrained to land: %w", err)
		}
		painted[geojson.LayerCivic] = civicPainted
	}

	// Paper base: fill the entire tile with a white texture so road cutouts show through.
	base := texture.TileTexture(g.textures[geojson.LayerPaper], g.tileSize, params.OffsetX, params.OffsetY)
	composited, err := composite.CompositeLayersOverBase(
		base,
		painted,
		[]geojson.LayerType{geojson.LayerWater, geojson.LayerLand, geojson.LayerParks, geojson.LayerCivic, geojson.LayerRoads, geojson.LayerHighways},
		g.tileSize,
	)
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
