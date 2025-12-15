package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MeKo-Tech/watercolormap/internal/datasource"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
)

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
