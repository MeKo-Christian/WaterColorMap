package renderer

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MeKo-Tech/watercolormap/internal/datasource"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
)

func assertPNGHasAnyNonTransparentPixel(t *testing.T, path string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open PNG %s: %v", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("Failed to decode PNG %s: %v", path, err)
	}

	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a != 0 {
				return
			}
		}
	}

	t.Fatalf("PNG is fully transparent: %s", path)
}

func decodePNGToNRGBA(t *testing.T, path string) *image.NRGBA {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open PNG %s: %v", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("Failed to decode PNG %s: %v", path, err)
	}

	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}

	return dst
}

func emptyTransparentTile(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	// image.NewNRGBA initializes to 0,0,0,0 which is transparent.
	return img
}

func borderAlphaMaskMatches(t *testing.T, a, b *image.NRGBA, orientation string) {
	t.Helper()

	if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
		t.Fatalf("Image size mismatch: A=%v B=%v", a.Bounds(), b.Bounds())
	}

	w := a.Bounds().Dx()
	h := a.Bounds().Dy()

	switch orientation {
	case "vertical": // compare right edge of A with left edge of B
		ax := w - 1
		bx := 0
		for y := 0; y < h; y++ {
			_, _, _, aa := a.At(ax, y).RGBA()
			_, _, _, ba := b.At(bx, y).RGBA()
			if (aa > 0) != (ba > 0) {
				t.Fatalf("Border alpha mismatch at y=%d (A.right alpha>0=%v, B.left alpha>0=%v)", y, aa > 0, ba > 0)
			}
		}
	case "horizontal": // compare bottom edge of A with top edge of B
		ay := h - 1
		by := 0
		for x := 0; x < w; x++ {
			_, _, _, aa := a.At(x, ay).RGBA()
			_, _, _, ba := b.At(x, by).RGBA()
			if (aa > 0) != (ba > 0) {
				t.Fatalf("Border alpha mismatch at x=%d (A.bottom alpha>0=%v, B.top alpha>0=%v)", x, aa > 0, ba > 0)
			}
		}
	default:
		t.Fatalf("Unknown orientation %q", orientation)
	}
}

func TestMultiPassRendererCreation(t *testing.T) {
	requireIntegration(t)

	stylesDir := "../../assets/styles"
	outputDir := "../../testdata/output/multipass"

	renderer, err := NewMultiPassRenderer(stylesDir, outputDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Close()

	t.Log("MultiPassRenderer created successfully")

	// Verify directories were created
	if _, err := os.Stat(renderer.outputDir); err != nil {
		t.Errorf("Output directory not created: %v", err)
	}
	if _, err := os.Stat(renderer.tempDir); err != nil {
		t.Errorf("Temp directory not created: %v", err)
	}
}

func TestRenderTileWithRealData(t *testing.T) {
	requireIntegration(t)

	// Create renderer
	stylesDir := "../../assets/styles"
	outputDir := "../../testdata/output/multipass"
	renderer, err := NewMultiPassRenderer(stylesDir, outputDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Close()

	// Fetch real OSM data for a test tile
	coords := tile.NewCoords(13, 4317, 2692) // Hanover test tile
	t.Logf("Fetching OSM data for tile %s", coords.String())

	ds := datasource.NewOverpassDataSource("")
	defer ds.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Fetch tile data
	tileData, err := ds.FetchTileData(ctx, types.TileCoordinate{
		Zoom: int(coords.Z),
		X:    int(coords.X),
		Y:    int(coords.Y),
	})
	if err != nil {
		t.Fatalf("Failed to fetch tile data: %v", err)
	}

	t.Logf("Fetched %d features: %v", tileData.Features.Count(), tileData.Features.FeatureCounts())

	// Render all layers
	t.Logf("Rendering all layers for tile %s", coords.String())
	result, err := renderer.RenderTile(coords, tileData)
	if err != nil {
		t.Fatalf("Failed to render tile: %v", err)
	}

	// Verify results
	t.Logf("Render results:")
	for layer, layerResult := range result.Layers {
		switch {
		case layerResult.Error != nil:
			t.Logf("  %s: ERROR - %v", layer, layerResult.Error)
		case layerResult.OutputPath == "":
			t.Logf("  %s: SKIPPED (no features)", layer)
		default:
			t.Logf("  %s: SUCCESS - %s", layer, layerResult.OutputPath)

			// Verify file exists
			if _, err := os.Stat(layerResult.OutputPath); err != nil {
				t.Errorf("Layer output file not found: %s", layerResult.OutputPath)
			}

			// Non-land layers should not be fully transparent when we rendered them
			if layer != geojson.LayerLand {
				assertPNGHasAnyNonTransparentPixel(t, layerResult.OutputPath)
			}
		}
	}

	// Verify at least land and water layers were rendered
	landResult := result.Layers[geojson.LayerLand]
	if landResult == nil || landResult.Error != nil {
		t.Errorf("Land layer should always render successfully")
	}

	// Check if water layer has features
	waterResult := result.Layers[geojson.LayerWater]
	if waterResult != nil && waterResult.Error != nil {
		t.Errorf("Water layer rendering failed: %v", waterResult.Error)
	}

	t.Logf("Multi-pass rendering completed successfully")
}

func TestRenderAdjacentTilesWithRealData(t *testing.T) {
	requireIntegration(t)

	stylesDir := "../../assets/styles"
	outputDir := "../../testdata/output/multipass"
	renderer, err := NewMultiPassRenderer(stylesDir, outputDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Close()

	ds := datasource.NewOverpassDataSource("")
	defer ds.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Render a small 3x3 grid around the Hannover reference tile so we can
	// validate basic edge consistency across neighbors.
	center := tile.NewCoords(13, 4317, 2692)
	var tiles []tile.Coords
	for dy := int32(-1); dy <= 1; dy++ {
		for dx := int32(-1); dx <= 1; dx++ {
			tiles = append(tiles, tile.NewCoords(center.Z, uint32(int32(center.X)+dx), uint32(int32(center.Y)+dy)))
		}
	}

	// Collect rendered outputs so we can compare edges afterwards.
	rendered := make(map[tile.Coords]map[geojson.LayerType]string)
	for _, coords := range tiles {
		coords := coords
		t.Run(coords.String(), func(t *testing.T) {
			tileData, err := ds.FetchTileData(ctx, types.TileCoordinate{Zoom: int(coords.Z), X: int(coords.X), Y: int(coords.Y)})
			if err != nil {
				t.Fatalf("Failed to fetch tile data: %v", err)
			}

			result, err := renderer.RenderTile(coords, tileData)
			if err != nil {
				t.Fatalf("Failed to render tile: %v", err)
			}

			// Land should always render
			land := result.Layers[geojson.LayerLand]
			if land == nil || land.Error != nil || land.OutputPath == "" {
				t.Fatalf("Land layer did not render")
			}

			paths := make(map[geojson.LayerType]string)
			for layer, lr := range result.Layers {
				if lr == nil {
					continue
				}
				if lr.Error != nil {
					t.Fatalf("Layer %s render error: %v", layer, lr.Error)
				}
				paths[layer] = lr.OutputPath
				if layer != geojson.LayerLand && lr.OutputPath != "" {
					assertPNGHasAnyNonTransparentPixel(t, lr.OutputPath)
				}
			}
			rendered[coords] = paths
		})
	}

	// Compare alpha masks on tile borders for each layer (treat skipped layers as fully transparent).
	layersToCompare := []geojson.LayerType{geojson.LayerWater, geojson.LayerParks, geojson.LayerCivic, geojson.LayerRoads}
	for _, coords := range tiles {
		coords := coords

		// Horizontal neighbor (east)
		east := tile.NewCoords(coords.Z, coords.X+1, coords.Y)
		if _, ok := rendered[east]; ok {
			for _, layer := range layersToCompare {
				layer := layer
				name := coords.String() + "_to_" + east.String() + "_" + string(layer) + "_vertical_border"
				t.Run(name, func(t *testing.T) {
					leftPath := rendered[coords][layer]
					rightPath := rendered[east][layer]

					leftImg := emptyTransparentTile(256)
					if leftPath != "" {
						leftImg = decodePNGToNRGBA(t, leftPath)
					}
					rightImg := emptyTransparentTile(256)
					if rightPath != "" {
						rightImg = decodePNGToNRGBA(t, rightPath)
					}

					borderAlphaMaskMatches(t, leftImg, rightImg, "vertical")
				})
			}
		}

		// Vertical neighbor (south)
		south := tile.NewCoords(coords.Z, coords.X, coords.Y+1)
		if _, ok := rendered[south]; ok {
			for _, layer := range layersToCompare {
				layer := layer
				name := coords.String() + "_to_" + south.String() + "_" + string(layer) + "_horizontal_border"
				t.Run(name, func(t *testing.T) {
					topPath := rendered[coords][layer]
					bottomPath := rendered[south][layer]

					topImg := emptyTransparentTile(256)
					if topPath != "" {
						topImg = decodePNGToNRGBA(t, topPath)
					}
					bottomImg := emptyTransparentTile(256)
					if bottomPath != "" {
						bottomImg = decodePNGToNRGBA(t, bottomPath)
					}

					borderAlphaMaskMatches(t, topImg, bottomImg, "horizontal")
				})
			}
		}
	}
}

func TestLayerPathHelpers(t *testing.T) {
	outputDir := "/tmp/test_tiles"
	coords := tile.NewCoords(13, 4297, 2754)

	// Test GetLayerPath
	path := GetLayerPath(outputDir, coords, geojson.LayerWater)
	expected := filepath.Join(outputDir, "z13_x4297_y2754_water.png")
	if path != expected {
		t.Errorf("GetLayerPath() = %s, want %s", path, expected)
	}

	// Test LayerExists (should return false for non-existent file)
	if LayerExists(outputDir, coords, geojson.LayerWater) {
		t.Error("LayerExists() should return false for non-existent file")
	}
}

func TestRenderLandLayerOnly(t *testing.T) {
	requireIntegration(t)

	// Test rendering just the land layer (no features required)
	stylesDir := "../../assets/styles"
	outputDir := "../../testdata/output/land_only"

	renderer, err := NewMultiPassRenderer(stylesDir, outputDir)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	defer renderer.Close()

	coords := tile.NewCoords(13, 4317, 2692)
	bounds := coords.BoundsMercator()

	// Render land layer
	stylePath := filepath.Join(stylesDir, "layers", "land.xml")
	result := renderer.renderLandLayer(coords, stylePath, bounds)

	if result.Error != nil {
		t.Fatalf("Failed to render land layer: %v", result.Error)
	}

	if result.OutputPath == "" {
		t.Error("Expected output path for land layer")
	}

	// Verify file exists
	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Errorf("Land layer output not found: %s", result.OutputPath)
	}

	t.Logf("Land layer rendered successfully: %s", result.OutputPath)
}
