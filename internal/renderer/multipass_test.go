package renderer

import (
	"context"
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

	tiles := []tile.Coords{
		tile.NewCoords(13, 4317, 2692), // center (Hannover)
		tile.NewCoords(13, 4318, 2692), // east
		tile.NewCoords(13, 4317, 2693), // south
	}

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

			// For any rendered non-land layer, ensure output isn't fully transparent.
			for layer, lr := range result.Layers {
				if layer == geojson.LayerLand {
					continue
				}
				if lr == nil {
					continue
				}
				if lr.Error != nil {
					t.Fatalf("Layer %s render error: %v", layer, lr.Error)
				}
				if lr.OutputPath == "" {
					// skipped (no features)
					continue
				}
				assertPNGHasAnyNonTransparentPixel(t, lr.OutputPath)
			}
		})
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
