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
	"sort"
	"strings"
	"sync"

	"github.com/MeKo-Tech/watercolormap/internal/composite"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/renderer"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/MeKo-Tech/watercolormap/internal/watercolor"
)

// StageCapture represents a single captured intermediate stage.
type StageCapture struct {
	Name        string      // e.g., "01_water_alpha"
	Description string      // e.g., "Alpha mask extracted from water layer"
	Image       image.Image // The actual image data
	ZOrder      int         // For sorting (01, 02, etc.)
}

// DebugContext optionally collects intermediate pipeline stages.
type DebugContext struct {
	Stages []StageCapture
	mu     sync.Mutex // Thread-safe
}

// Capture adds a stage to the debug context if it exists.
func (dc *DebugContext) Capture(name, description string, img image.Image, zorder int) {
	if dc == nil {
		return // Fast path: no debug context, no overhead
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.Stages = append(dc.Stages, StageCapture{
		Name:        name,
		Description: description,
		Image:       img,
		ZOrder:      zorder,
	})
}

// SortedStages returns stages sorted by ZOrder.
func (dc *DebugContext) SortedStages() []StageCapture {
	if dc == nil {
		return nil
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()

	sorted := make([]StageCapture, len(dc.Stages))
	copy(sorted, dc.Stages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ZOrder < sorted[j].ZOrder
	})
	return sorted
}

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
// debugCtx can be *DebugContext or nil; pass nil in production for zero overhead.
func (g *Generator) Generate(ctx context.Context, coords tile.Coords, force bool, filenameSuffix string, debugCtx interface{}) (string, string, error) {
	// Type-assert debugCtx to *DebugContext if provided
	var dc *DebugContext
	if debugCtx != nil {
		dc = debugCtx.(*DebugContext)
	}
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

	// Generate Perlin noise once for all layers to avoid redundant allocations
	params.PerlinNoise = mask.GeneratePerlinNoiseWithOffset(
		params.TileSize, params.TileSize,
		params.NoiseScale, params.Seed,
		params.OffsetX, params.OffsetY,
	)

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

	// Capture raw rendered layers
	for layer, img := range raw {
		if img != nil {
			dc.Capture(
				fmt.Sprintf("00_rendered_%s", layer),
				fmt.Sprintf("Raw rendered %s layer from Mapnik", layer),
				img,
				0,
			)
		}
	}

	// --- Phase 3 (revised): cross-layer mask construction ---
	// water = polygonal water bodies, rivers = linear waterways
	waterImg := raw[geojson.LayerWater]
	riversImg := raw[geojson.LayerRivers]
	roadsImg := raw[geojson.LayerRoads]
	highwaysImg := raw[geojson.LayerHighways]

	baseBounds := image.Rect(0, 0, params.TileSize, params.TileSize)
	waterMask := mask.NewEmptyMask(baseBounds)
	riversMask := mask.NewEmptyMask(baseBounds)
	roadsMask := mask.NewEmptyMask(baseBounds)
	if waterImg != nil {
		waterMask = mask.ExtractAlphaMask(waterImg)
	}
	if riversImg != nil {
		riversMask = mask.ExtractAlphaMask(riversImg)
	}
	if roadsImg != nil {
		roadsMask = mask.ExtractAlphaMask(roadsImg)
	}

	// Capture alpha masks
	dc.Capture("01_water_alpha", "Alpha mask from water layer", waterMask, 1)
	dc.Capture("02_rivers_alpha", "Alpha mask from rivers layer", riversMask, 2)
	dc.Capture("03_roads_alpha", "Alpha mask from roads layer", roadsMask, 3)

	// Add highways alpha extraction explicitly
	highwaysAlpha := mask.NewEmptyMask(baseBounds)
	if highwaysImg != nil {
		highwaysAlpha = mask.ExtractAlphaMask(highwaysImg)
	}
	dc.Capture("03_highways_alpha", "Alpha mask from highways layer", highwaysAlpha, 3)

	// Include both water bodies and rivers in the non-land mask
	nonLandBase := mask.MaxMask(mask.MaxMask(waterMask, riversMask), roadsMask)
	dc.Capture("04_nonland_union", "Union of water + rivers + roads masks", nonLandBase, 4)

	// Paint water from its own alpha mask (not the combined non-land mask).
	if waterImg != nil {
		waterPainted, err := watercolor.PaintLayer(waterImg, geojson.LayerWater, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint water: %w", err)
		}
		painted[geojson.LayerWater] = waterPainted
		dc.Capture("12_painted_water", "Watercolor-painted water layer", waterPainted, 12)
	}

	// Paint rivers from their own alpha mask
	if riversImg != nil {
		riversPainted, err := watercolor.PaintLayer(riversImg, geojson.LayerRivers, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint rivers: %w", err)
		}
		painted[geojson.LayerRivers] = riversPainted
		dc.Capture("13_painted_rivers", "Watercolor-painted rivers layer", riversPainted, 18)
	}

	// Build nonLand mask using the standard mask pipeline (blur/noise/threshold/AA), then invert for land.
	// We reuse the same parameters as other layers (but operate on the combined mask).
	landMask, err := func() (*image.Gray, error) {
		blurred := mask.BoxBlurSigma(nonLandBase, params.BlurSigma)
		dc.Capture("05_blur", "Blurred non-land mask", blurred, 4)

		dc.Capture("05_noise", "Perlin noise texture (reference)", params.PerlinNoise, 5)

		noisy := blurred
		if params.NoiseStrength != 0 {
			noisy = mask.ApplyNoiseToMask(blurred, params.PerlinNoise, params.NoiseStrength)
			dc.Capture("07_noisy", "Blurred mask with Perlin noise applied", noisy, 6)
		}
		finalMask := mask.ApplyThresholdWithAntialiasAndInvert(noisy, params.Threshold)
		dc.Capture("08_threshold_antialias", "Thresholded and antialiased non-land mask", finalMask, 7)
		return finalMask, nil
	}()
	if err != nil {
		return "", "", fmt.Errorf("failed to process non-land mask: %w", err)
	}

	// Paint land directly from derived land mask.
	paintedLandRaw, err := watercolor.PaintLayerFromFinalMask(landMask, geojson.LayerLand, params)
	if err != nil {
		return "", "", fmt.Errorf("failed to paint land from derived mask: %w", err)
	}

	// Compute distance-based edge mask to avoid stacking in small features
	// Radius of 9 pixels controls how far the darkening effect extends from edges
	// Gamma of 9.0 creates steep falloff concentrated near edges
	landMaskShadow := mask.CreateDistanceEdgeMask(landMask, 9.0, 9.0)
	dc.Capture("09_land_mask_shadow", "Distance-based edge mask", landMaskShadow, 9)

	// Apply soft edge mask to darken edges while preserving alpha and color information
	// Strength 0.3-0.5 provides subtle darkening without going to black
	paintedLand := mask.ApplySoftEdgeMask(paintedLandRaw, landMaskShadow, 0.3)
	painted[geojson.LayerLand] = paintedLand
	dc.Capture("10_painted_land", "Watercolor-painted land layer with soft edges", paintedLand, 10)

	// Create composite of land on white canvas for debugging
	whiteCanvas := texture.TileTexture(g.textures[geojson.LayerPaper], params.TileSize, params.OffsetX, params.OffsetY)
	landOnCanvas, err := composite.CompositeLayersOverBase(
		whiteCanvas,
		map[geojson.LayerType]image.Image{geojson.LayerLand: paintedLand},
		[]geojson.LayerType{geojson.LayerLand},
		params.TileSize,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to composite land on canvas: %w", err)
	}
	dc.Capture("11_painted_land_on_canvas", "Land layer composited on white canvas", landOnCanvas, 11)

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
		dc.Capture("15_painted_roads", "Watercolor-painted roads layer", roadsPainted, 15)
	}

	// Paint highways/major roads on top.
	if highwaysImg != nil {
		highwaysPainted, err := watercolor.PaintLayer(highwaysImg, geojson.LayerHighways, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint highways: %w", err)
		}
		painted[geojson.LayerHighways] = highwaysPainted
		dc.Capture("19_painted_highways", "Watercolor-painted highways layer", highwaysPainted, 19)
	}

	// Constrain parks/civic/buildings to land, then paint.
	if parksImg := raw[geojson.LayerParks]; parksImg != nil {
		parksMask := mask.MinMask(mask.ExtractAlphaMask(parksImg), landMask)
		dc.Capture("14_parks_on_land", "Parks constrained to land", parksMask, 14)
		parksPainted, err := watercolor.PaintLayerFromMask(parksMask, geojson.LayerParks, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint parks constrained to land: %w", err)
		}
		painted[geojson.LayerParks] = parksPainted
		dc.Capture("16_painted_parks", "Watercolor-painted parks layer", parksPainted, 16)
	}
	if civicImg := raw[geojson.LayerCivic]; civicImg != nil {
		civicMask := mask.MinMask(mask.ExtractAlphaMask(civicImg), landMask)
		dc.Capture("10_civic_on_land", "Civic constrained to land", civicMask, 10)
		civicPainted, err := watercolor.PaintLayerFromMask(civicMask, geojson.LayerCivic, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint civic constrained to land: %w", err)
		}
		painted[geojson.LayerCivic] = civicPainted
		dc.Capture("17_painted_civic", "Watercolor-painted civic layer", civicPainted, 17)
	}
	if buildingsImg := raw[geojson.LayerBuildings]; buildingsImg != nil {
		buildingsMask := mask.MinMask(mask.ExtractAlphaMask(buildingsImg), landMask)
		dc.Capture("11_buildings_on_land", "Buildings constrained to land", buildingsMask, 11)
		buildingsPainted, err := watercolor.PaintLayerFromMask(buildingsMask, geojson.LayerBuildings, params)
		if err != nil {
			return "", "", fmt.Errorf("failed to paint buildings constrained to land: %w", err)
		}
		painted[geojson.LayerBuildings] = buildingsPainted
		dc.Capture("18_painted_buildings", "Watercolor-painted buildings layer", buildingsPainted, 18)
	}

	// Paper base: fill the entire tile with a white texture so road cutouts show through.
	base := texture.TileTexture(g.textures[geojson.LayerPaper], params.TileSize, params.OffsetX, params.OffsetY)
	// Layer order matches OSM standard: land (back) → parks → rivers → water → roads → highways → buildings → civic (front)
	composited, err := composite.CompositeLayersOverBase(
		base,
		painted,
		[]geojson.LayerType{geojson.LayerLand, geojson.LayerParks, geojson.LayerRivers, geojson.LayerWater, geojson.LayerRoads, geojson.LayerHighways, geojson.LayerBuildings, geojson.LayerCivic},
		params.TileSize,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to composite layers: %w", err)
	}
	dc.Capture("20_combined_metatile", "Composited layers (before crop)", composited, 20)

	// Crop back to the requested tile size.
	final := composited
	if padPx > 0 {
		cropRect := image.Rect(padPx, padPx, padPx+g.tileSize, padPx+g.tileSize)
		final = cropNRGBA(composited, cropRect)
	}
	dc.Capture("21_combined_final", "Final tile (after crop)", final, 21)

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
