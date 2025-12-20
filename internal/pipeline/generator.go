package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeKo-Tech/watercolormap/internal/composite"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/renderer"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/MeKo-Tech/watercolormap/internal/watercolor"
)

// GeneratorOptions controls output and encoding behavior.
type GeneratorOptions struct {
	// PNGCompression controls PNG encoding. Supported values:
	// "default", "speed", "best", "none".
	PNGCompression string

	// TileWriter optionally writes tiles to an alternative storage backend (e.g., MBTiles).
	// If nil, tiles are written to disk in outputDir.
	TileWriter TileWriter

	// FolderStructure controls file naming for folder format. Supported values:
	// "flat" (z{z}_x{x}_y{y}.png), "nested" ({z}/{x}/{y}.png).
	FolderStructure string
}

// TileWriter writes tile data to a storage backend.
type TileWriter interface {
	WriteTile(z, x, y int, pngData []byte) error
}

// DataSource fetches OSM features for a tile coordinate.
type DataSource interface {
	FetchTileData(context.Context, types.TileCoordinate) (*types.TileData, error)
}

type dataSourceWithBounds interface {
	FetchTileDataWithBounds(context.Context, types.TileCoordinate, types.BoundingBox) (*types.TileData, error)
}

// Generator wires datasource, rendering, watercolor, and compositing into a single step.
type Generator struct {
	ds         DataSource
	textures   map[geojson.LayerType]image.Image
	logger     *slog.Logger
	options    GeneratorOptions
	stylesDir  string
	outputDir  string
	tileSize   int
	seed       int64
	keepLayers bool
}

// NewGenerator loads textures and prepares a generator.
func NewGenerator(ds DataSource, stylesDir, texturesDir, outputDir string, tileSize int, seed int64, keepLayers bool, logger *slog.Logger, opts GeneratorOptions) (*Generator, error) {
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
		options:    opts,
	}, nil
}

// Generate renders, paints, composites, and writes the final tile PNG.
// Returns the final tile path and (optionally) the layer directory when kept.
func (g *Generator) Generate(ctx context.Context, coords tile.Coords, force bool, filenameSuffix string) (string, string, error) {
	suffix := strings.TrimSpace(filenameSuffix)

	// Compute final path based on folder structure setting
	var finalPath string
	var tileDir string
	if g.options.FolderStructure == "nested" {
		// Nested structure: {z}/{x}/{y}.png
		z := fmt.Sprintf("%d", coords.Z)
		x := fmt.Sprintf("%d", coords.X)
		y := fmt.Sprintf("%d", coords.Y)
		tileDir = filepath.Join(g.outputDir, z, x)
		finalPath = filepath.Join(tileDir, y+suffix+".png")
	} else {
		// Flat structure (default): z{z}_x{x}_y{y}.png
		finalPath = filepath.Join(g.outputDir, fmt.Sprintf("%s%s.png", coords.String(), suffix))
		tileDir = g.outputDir
	}

	if !force {
		if _, err := os.Stat(finalPath); err == nil {
			g.log().Info("Tile already exists; skipping", "coords", coords.String(), "path", finalPath)
			return finalPath, "", nil
		}
	}

	if err := os.MkdirAll(tileDir, 0o755); err != nil {
		return "", "", fmt.Errorf("failed to create output dir: %w", err)
	}

	params := watercolor.DefaultParams(g.tileSize, g.seed, g.textures)

	// Adjust blur sigma based on zoom level for sharper edges at higher zooms
	params.BlurSigma = watercolor.ZoomAdjustedBlurSigma(params.BlurSigma, int(coords.Z))
	params.AntialiasSigma = watercolor.ZoomAdjustedBlurSigma(params.AntialiasSigma, int(coords.Z))

	padPx := watercolor.RequiredPaddingPx(params)
	if padPx > g.tileSize {
		padPx = g.tileSize
	}

	// Switch the pipeline to operate on a padded metatile to avoid edge artifacts
	// from blurs/edge halos; we'll crop back to the requested tile at the end.
	metatileSize := g.tileSize + 2*padPx
	params.TileSize = metatileSize
	params.OffsetX = int(coords.X)*g.tileSize - padPx
	params.OffsetY = int(coords.Y)*g.tileSize - padPx

	g.log().Info("Fetching tile data", "coords", coords.String(), "padPx", padPx)
	tileCoord := types.TileCoordinate{
		Zoom: int(coords.Z),
		X:    int(coords.X),
		Y:    int(coords.Y),
	}

	dataBounds := types.TileToBounds(tileCoord)
	if padPx > 0 {
		padFrac := float64(padPx) / float64(g.tileSize)
		dataBounds = dataBounds.ExpandByFraction(padFrac)
	}

	var data *types.TileData
	var err error
	if dsb, ok := g.ds.(dataSourceWithBounds); ok {
		data, err = dsb.FetchTileDataWithBounds(ctx, tileCoord, dataBounds)
	} else {
		data, err = g.ds.FetchTileData(ctx, tileCoord)
	}
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
	mpRenderer, err := renderer.NewMultiPassRenderer(g.stylesDir, layerDir, g.tileSize, padPx)
	if err != nil {
		return "", "", fmt.Errorf("failed to create multipass renderer: %w", err)
	}
	defer mpRenderer.Close() // nolint:errcheck

	renderResult, err := mpRenderer.RenderTile(coords, data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render layers: %w", err)
	}

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

	baseBounds := image.Rect(0, 0, params.TileSize, params.TileSize)
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

	// Constrain parks/civic/buildings to land, then paint.
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
	if buildingsImg := raw[geojson.LayerBuildings]; buildingsImg != nil {
		buildingsMask := mask.MinMask(mask.ExtractAlphaMask(buildingsImg), landMask)
		buildingsPainted, err := watercolor.PaintLayerFromMask(buildingsMask, geojson.LayerBuildings, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint buildings constrained to land: %w", err)
		}
		painted[geojson.LayerBuildings] = buildingsPainted
	}

	// Paper base: fill the entire tile with a white texture so road cutouts show through.
	base := texture.TileTexture(g.textures[geojson.LayerPaper], params.TileSize, params.OffsetX, params.OffsetY)
	composited, err := composite.CompositeLayersOverBase(
		base,
		painted,
		[]geojson.LayerType{geojson.LayerWater, geojson.LayerLand, geojson.LayerParks, geojson.LayerCivic, geojson.LayerBuildings, geojson.LayerRoads, geojson.LayerHighways},
		params.TileSize,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to composite layers: %w", err)
	}

	// Crop back to the requested tile size.
	final := composited
	if padPx > 0 {
		cropRect := image.Rect(padPx, padPx, padPx+g.tileSize, padPx+g.tileSize)
		final = cropNRGBA(composited, cropRect)
	}

	// Encode PNG to get the data
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	switch strings.ToLower(strings.TrimSpace(g.options.PNGCompression)) {
	case "", "default":
		enc.CompressionLevel = png.DefaultCompression
	case "speed", "fast", "best-speed":
		enc.CompressionLevel = png.BestSpeed
	case "best", "best-compression":
		enc.CompressionLevel = png.BestCompression
	case "none", "no", "nocompression", "no-compression":
		enc.CompressionLevel = png.NoCompression
	default:
		// Keep default on unknown values.
		enc.CompressionLevel = png.DefaultCompression
	}

	// Use TileWriter if provided, otherwise write to disk
	if g.options.TileWriter != nil {
		// Encode to bytes buffer
		var buf bytes.Buffer
		if err := enc.Encode(&buf, final); err != nil {
			return "", "", fmt.Errorf("failed to encode tile: %w", err)
		}

		// Write through TileWriter interface
		g.log().Info("Writing tile via TileWriter", "coords", coords.String())
		if err := g.options.TileWriter.WriteTile(int(coords.Z), int(coords.X), int(coords.Y), buf.Bytes()); err != nil {
			return "", "", fmt.Errorf("failed to write tile: %w", err)
		}

		// Return virtual path (no actual file)
		return finalPath, layerDirReturn, nil
	}

	// Traditional file output
	g.log().Info("Writing final tile", "coords", coords.String(), "path", finalPath)
	outFile, err := os.Create(finalPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create tile file: %w", err)
	}
	defer outFile.Close() // nolint:errcheck

	if err := enc.Encode(outFile, final); err != nil {
		return "", "", fmt.Errorf("failed to encode final tile: %w", err)
	}

	return finalPath, layerDirReturn, nil
}

func cropNRGBA(src image.Image, rect image.Rectangle) *image.NRGBA {
	if src == nil {
		return nil
	}
	if rect.Empty() {
		return image.NewNRGBA(image.Rect(0, 0, 0, 0))
	}
	if !rect.In(src.Bounds()) {
		// Best effort: intersect and return what we can.
		rect = rect.Intersect(src.Bounds())
	}

	dst := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			dst.Set(x, y, src.At(rect.Min.X+x, rect.Min.Y+y))
		}
	}
	return dst
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
