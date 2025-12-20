package watercolor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/MeKo-Christian/go-overpass"
	"github.com/MeKo-Tech/watercolormap/internal/composite"
	"github.com/MeKo-Tech/watercolormap/internal/datasource"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/renderer"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
)

func requireIntegrationLocal(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	if os.Getenv("WATERCOLORMAP_INTEGRATION") != "1" {
		t.Skip("skipping integration test (set WATERCOLORMAP_INTEGRATION=1 to enable)")
	}
}

func TestWatercolorStagesGolden_HannoverRealTile(t *testing.T) {
	requireIntegrationLocal(t)

	// This test renders a real Hannover tile (OSM via Overpass + Mapnik), then writes
	// all intermediate stages as PNGs. It's meant for human debugging and visual review.
	//
	// It is opt-in because it depends on:
	// - Mapnik being available
	// - network availability (Overpass)
	// - OSM data changing over time
	//
	// Default behavior: write debug outputs for human inspection.
	//
	// Update goldens:
	//   UPDATE_GOLDEN=1 WATERCOLORMAP_INTEGRATION=1 go test ./... -run TestWatercolorStagesGolden_HannoverRealTile
	//
	// Compare against goldens (may be flaky as OSM data changes):
	//   WATERCOLORMAP_COMPARE_GOLDEN=1 WATERCOLORMAP_INTEGRATION=1 go test ./... -run TestWatercolorStagesGolden_HannoverRealTile

	goldenDir := filepath.Join("..", "..", "testdata", "golden", "watercolor-stages-hannover")
	debugRoot := filepath.Join("..", "..", "testdata", "output", "watercolor-stages-hannover")

	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("failed to create golden dir: %v", err)
	}
	if err := os.MkdirAll(debugRoot, 0o755); err != nil {
		t.Fatalf("failed to create debug root dir: %v", err)
	}

	update := os.Getenv("UPDATE_GOLDEN") == "1"
	compare := os.Getenv("WATERCOLORMAP_COMPARE_GOLDEN") == "1"

	base := tile.NewCoords(13, 4317, 2692)
	centerLon, centerLat := base.Center()
	center := orb.Point{centerLon, centerLat}

	coordsCases := make([]tile.Coords, 0, 5)
	for z := uint32(11); z <= 15; z++ {
		t := maptile.At(center, maptile.Zoom(z))
		coordsCases = append(coordsCases, tile.NewCoords(z, t.X, t.Y))
	}

	tileSize := 256
	seed := int64(123)

	// Fetch + render per-zoom, sharing the same datasource/textures.
	stylesDir := filepath.Join("..", "..", "assets", "styles")
	texturesDir := filepath.Join("..", "..", "assets", "textures")

	textures, err := texture.LoadDefaultTextures(texturesDir)
	if err != nil {
		t.Fatalf("failed to load textures: %v", err)
	}

	// Calculate padding early to use in data fetch and rendering.
	// This prevents polygon clipping at tile boundaries by rendering a larger
	// "metatile" and cropping back to the final tile size.
	baseParams := DefaultParams(tileSize, seed, textures)
	padPx := RequiredPaddingPx(baseParams)
	metatileSize := tileSize + 2*padPx

	ds := datasource.NewOverpassDataSource("").WithRawResponseStorage(true)
	defer ds.Close()

	for _, coords := range coordsCases {
		coords := coords
		caseName := coords.String()
		debugDir := filepath.Join(debugRoot, caseName)
		if err := os.MkdirAll(debugDir, 0o755); err != nil {
			t.Fatalf("failed to create debug dir: %v", err)
		}

		// Fetch data with expanded bounds to get polygons that cross tile edges.
		tileData, err := func() (*types.TileData, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			tileCoord := types.TileCoordinate{Zoom: int(coords.Z), X: int(coords.X), Y: int(coords.Y)}
			dataBounds := types.TileToBounds(tileCoord)
			if padPx > 0 {
				padFrac := float64(padPx) / float64(tileSize)
				dataBounds = dataBounds.ExpandByFraction(padFrac)
			}
			return ds.FetchTileDataWithBounds(ctx, tileCoord, dataBounds)
		}()
		if err != nil {
			t.Fatalf("failed to fetch tile data for %s: %v", caseName, err)
		}

		// Save water-only subset of Overpass API response for debugging (to reduce file size)
		if tileData.OverpassResult != nil {
			waterOnlyResult := extractWaterElements(tileData.OverpassResult)
			overpassJSON, err := json.MarshalIndent(waterOnlyResult, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal Overpass result for %s: %v", caseName, err)
			}
			overpassPath := filepath.Join(debugDir, "00_overpass_water_only.json")
			if err := os.WriteFile(overpassPath, overpassJSON, 0o644); err != nil {
				t.Fatalf("failed to write Overpass response for %s: %v", caseName, err)
			}
			t.Logf("Saved water-only Overpass data to: %s (reduced from %d total elements)",
				overpassPath, len(tileData.OverpassResult.Ways)+len(tileData.OverpassResult.Relations))
		}

		renderDir := filepath.Join(debugDir, "rendered")
		if err := os.MkdirAll(renderDir, 0o755); err != nil {
			t.Fatalf("failed to create render debug dir: %v", err)
		}

		// Render metatile (larger than final tile) to avoid polygon clipping at edges.
		mpRenderer, err := renderer.NewMultiPassRenderer(stylesDir, renderDir, tileSize, padPx)
		if err != nil {
			t.Fatalf("failed to create multipass renderer: %v", err)
		}

		renderResult, err := mpRenderer.RenderTile(coords, tileData)
		_ = mpRenderer.Close()
		if err != nil {
			t.Fatalf("failed to render layers for %s: %v", caseName, err)
		}

		readLayer := func(layer geojson.LayerType) image.Image {
			lr := renderResult.Layers[layer]
			if lr == nil || lr.Error != nil || lr.OutputPath == "" {
				return nil
			}
			f, err := os.Open(lr.OutputPath)
			if err != nil {
				t.Fatalf("failed to open rendered layer %s (%s): %v", layer, caseName, err)
			}
			defer f.Close()
			img, err := png.Decode(f)
			if err != nil {
				t.Fatalf("failed to decode rendered layer %s (%s): %v", layer, caseName, err)
			}
			return img
		}

		waterImg := readLayer(geojson.LayerWater)
		roadsImg := readLayer(geojson.LayerRoads)
		highwaysImg := readLayer(geojson.LayerHighways)
		parksImg := readLayer(geojson.LayerParks)
		civicImg := readLayer(geojson.LayerCivic)

		// Configure params for metatile rendering with adjusted offsets.
		params := DefaultParams(metatileSize, seed, textures)
		params.OffsetX = int(coords.X)*tileSize - padPx
		params.OffsetY = int(coords.Y)*tileSize - padPx

		baseBounds := image.Rect(0, 0, metatileSize, metatileSize)
		waterAlpha := mask.NewEmptyMask(baseBounds)
		roadsAlpha := mask.NewEmptyMask(baseBounds)
		highwaysAlpha := mask.NewEmptyMask(baseBounds)
		parksAlpha := mask.NewEmptyMask(baseBounds)
		civicAlpha := mask.NewEmptyMask(baseBounds)

		if waterImg != nil {
			waterAlpha = mask.ExtractAlphaMask(waterImg)
		}
		if roadsImg != nil {
			roadsAlpha = mask.ExtractAlphaMask(roadsImg)
		}
		if highwaysImg != nil {
			highwaysAlpha = mask.ExtractAlphaMask(highwaysImg)
		}
		if parksImg != nil {
			parksAlpha = mask.ExtractAlphaMask(parksImg)
		}
		if civicImg != nil {
			civicAlpha = mask.ExtractAlphaMask(civicImg)
		}

		nonLandBase := mask.MaxMask(waterAlpha, roadsAlpha)
		blur1 := mask.GaussianBlur(nonLandBase, params.BlurSigma)
		noise := mask.GeneratePerlinNoiseWithOffset(metatileSize, metatileSize, params.NoiseScale, params.Seed, params.OffsetX, params.OffsetY)
		noisy := blur1
		if params.NoiseStrength != 0 {
			noisy = mask.ApplyNoiseToMask(blur1, noise, params.NoiseStrength)
		}
		thresholded := mask.ApplyThreshold(noisy, params.Threshold)
		aa := thresholded
		if params.AntialiasSigma > 0 {
			aa = mask.AntialiasEdges(thresholded, params.AntialiasSigma)
		}
		landMask := mask.InvertMask(aa)

		parksOnLand := mask.MinMask(parksAlpha, landMask)
		civicOnLand := mask.MinMask(civicAlpha, landMask)

		paintedLand, err := PaintLayerFromFinalMask(landMask, geojson.LayerLand, params)
		if err != nil {
			t.Fatalf("PaintLayerFromFinalMask(land) failed (%s): %v", caseName, err)
		}
		paintedWater, err := PaintLayerFromMask(waterAlpha, geojson.LayerWater, params)
		if err != nil {
			t.Fatalf("PaintLayerFromMask(water) failed (%s): %v", caseName, err)
		}
		paintedHighways, err := PaintLayerFromMask(highwaysAlpha, geojson.LayerHighways, params)
		if err != nil {
			t.Fatalf("PaintLayerFromMask(highways) failed (%s): %v", caseName, err)
		}
		paintedParks, err := PaintLayerFromMask(parksOnLand, geojson.LayerParks, params)
		if err != nil {
			t.Fatalf("PaintLayerFromMask(parks) failed (%s): %v", caseName, err)
		}
		paintedCivic, err := PaintLayerFromMask(civicOnLand, geojson.LayerCivic, params)
		if err != nil {
			t.Fatalf("PaintLayerFromMask(civic) failed (%s): %v", caseName, err)
		}

		base := texture.TileTexture(textures[geojson.LayerPaper], metatileSize, params.OffsetX, params.OffsetY)
		combined, err := composite.CompositeLayersOverBase(
			base,
			map[geojson.LayerType]image.Image{
				geojson.LayerWater:    paintedWater,
				geojson.LayerLand:     paintedLand,
				geojson.LayerParks:    paintedParks,
				geojson.LayerCivic:    paintedCivic,
				geojson.LayerHighways: paintedHighways,
			},
			[]geojson.LayerType{geojson.LayerWater, geojson.LayerLand, geojson.LayerParks, geojson.LayerCivic, geojson.LayerHighways},
			metatileSize,
		)
		if err != nil {
			t.Fatalf("CompositeLayers failed (%s): %v", caseName, err)
		}

		// Crop all stages back to final tile size (remove padding).
		cropRect := image.Rect(padPx, padPx, padPx+tileSize, padPx+tileSize)
		crop := func(img image.Image) image.Image {
			if img == nil {
				return nil
			}
			return cropImage(img, cropRect)
		}

		stages := map[string]image.Image{
			"00_rendered_water.png":    crop(waterImg),
			"00_rendered_roads.png":    crop(roadsImg),
			"00_rendered_highways.png": crop(highwaysImg),
			"00_rendered_parks.png":    crop(parksImg),
			"00_rendered_civic.png":    crop(civicImg),
			"01_water_alpha.png":       crop(waterAlpha),
			"02_roads_alpha.png":       crop(roadsAlpha),
			"03_highways_alpha.png":    crop(highwaysAlpha),
			"04_nonland_union.png":     crop(nonLandBase),
			"04_blur.png":              crop(blur1),
			"05_noise.png":             crop(noise),
			"06_noisy.png":             crop(noisy),
			"07_threshold.png":         crop(thresholded),
			"08_antialias.png":         crop(aa),
			"09_land_inverted.png":     crop(landMask),
			"10_parks_on_land.png":     crop(parksOnLand),
			"11_civic_on_land.png":     crop(civicOnLand),
			"12_painted_water.png":     crop(paintedWater),
			"13_painted_land.png":      crop(paintedLand),
			"14_painted_parks.png":     crop(paintedParks),
			"15_painted_civic.png":     crop(paintedCivic),
			"16_painted_highways.png":  crop(paintedHighways),
			"17_combined.png":          crop(combined),
		}

		keys := make([]string, 0, len(stages))
		for k := range stages {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			img := stages[name]
			if img == nil {
				continue
			}
			if err := writePNG(filepath.Join(debugDir, name), img); err != nil {
				t.Fatalf("failed to write debug png %s (%s): %v", name, caseName, err)
			}
		}

		// Generate comparison report
		if tileData.OverpassResult != nil {
			reportPath := filepath.Join(debugDir, "00_analysis.txt")
			report := generateAnalysisReport(tileData, waterAlpha, parksAlpha, civicAlpha)
			if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
				t.Fatalf("failed to write analysis report for %s: %v", caseName, err)
			}
			t.Logf("Generated analysis report: %s", reportPath)
		}

		if !update && !compare {
			continue
		}

		for _, name := range keys {
			img := stages[name]
			if img == nil {
				continue
			}
			goldenPath := filepath.Join(goldenDir, caseName+"__"+name)

			if update {
				if err := writePNG(goldenPath, img); err != nil {
					t.Fatalf("failed to update golden %s (%s): %v", name, caseName, err)
				}
				continue
			}

			exists, err := fileExists(goldenPath)
			if err != nil {
				t.Fatalf("failed to stat golden %s: %v", goldenPath, err)
			}
			if !exists {
				t.Fatalf("missing golden %s; run: UPDATE_GOLDEN=1 WATERCOLORMAP_INTEGRATION=1 go test ./... -run TestWatercolorStagesGolden_HannoverRealTile", goldenPath)
			}

			gotBytes, err := encodePNG(img)
			if err != nil {
				t.Fatalf("failed to encode generated %s (%s): %v", name, caseName, err)
			}
			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden %s: %v", goldenPath, err)
			}

			gotImg, err := png.Decode(bytes.NewReader(gotBytes))
			if err != nil {
				t.Fatalf("failed to decode generated png %s (%s): %v", name, caseName, err)
			}
			wantImg, err := png.Decode(bytes.NewReader(wantBytes))
			if err != nil {
				t.Fatalf("failed to decode golden png %s (%s): %v", name, caseName, err)
			}

			if !imagesEqual(gotImg, wantImg) {
				t.Fatalf("golden mismatch for %s (%s)\n- golden: %s\n- got: %s\nTo update: UPDATE_GOLDEN=1 WATERCOLORMAP_INTEGRATION=1 go test ./... -run TestWatercolorStagesGolden_HannoverRealTile",
					name,
					caseName,
					goldenPath,
					filepath.Join(debugDir, name),
				)
			}
		}
	}
}

// cropImage crops an image to the specified rectangle.
func cropImage(src image.Image, rect image.Rectangle) image.Image {
	if src == nil {
		return nil
	}
	if rect.Empty() {
		return image.NewNRGBA(image.Rect(0, 0, 0, 0))
	}
	if !rect.In(src.Bounds()) {
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

// generateAnalysisReport creates a detailed analysis comparing Overpass data with rendered output
func generateAnalysisReport(tileData *types.TileData, waterAlpha, parksAlpha, civicAlpha *image.Gray) string {
	var report bytes.Buffer

	report.WriteString("=== WaterColorMap Rendering Analysis ===\n\n")

	// Tile information
	report.WriteString("TILE INFORMATION:\n")
	report.WriteString("  Coordinate: " + tileData.Coordinate.String() + "\n")
	report.WriteString("  Bounds: " + tileData.Bounds.String() + "\n")
	report.WriteString("  Fetched at: " + tileData.FetchedAt.Format(time.RFC3339) + "\n")
	report.WriteString("  Source: " + tileData.Source + "\n\n")

	// Overpass data statistics
	if tileData.OverpassResult != nil {
		result := tileData.OverpassResult
		report.WriteString("OVERPASS API DATA:\n")

		totalElements := len(result.Nodes) + len(result.Ways) + len(result.Relations)
		report.WriteString(fmt.Sprintf("  Total elements: %d\n", totalElements))
		report.WriteString(fmt.Sprintf("    Nodes: %d\n", len(result.Nodes)))
		report.WriteString(fmt.Sprintf("    Ways: %d\n", len(result.Ways)))
		report.WriteString(fmt.Sprintf("    Relations: %d\n\n", len(result.Relations)))

		// Water-specific analysis
		waterWays := 0
		waterRelations := 0

		for _, way := range result.Ways {
			if isWaterElement(way.Tags) {
				waterWays++
			}
		}

		for _, relation := range result.Relations {
			if isWaterElement(relation.Tags) {
				waterRelations++
			}
		}

		report.WriteString("  Water elements:\n")
		report.WriteString(fmt.Sprintf("    Ways: %d\n", waterWays))
		report.WriteString(fmt.Sprintf("    Relations: %d\n\n", waterRelations))
	}

	// Feature collection statistics
	report.WriteString("PROCESSED FEATURES:\n")
	counts := tileData.Features.FeatureCounts()
	report.WriteString(fmt.Sprintf("  Water: %d\n", counts["water"]))
	report.WriteString(fmt.Sprintf("  Parks: %d\n", counts["parks"]))
	report.WriteString(fmt.Sprintf("  Civic: %d\n", counts["civic"]))
	report.WriteString(fmt.Sprintf("  Roads: %d\n", counts["roads"]))
	report.WriteString(fmt.Sprintf("  Buildings: %d\n", counts["buildings"]))
	report.WriteString(fmt.Sprintf("  Total: %d\n\n", counts["total"]))

	// Rendered mask statistics
	if waterAlpha != nil {
		waterPixels := countNonZeroPixels(waterAlpha)
		totalPixels := waterAlpha.Bounds().Dx() * waterAlpha.Bounds().Dy()
		waterPct := float64(waterPixels) * 100.0 / float64(totalPixels)

		report.WriteString("RENDERED OUTPUT (water_alpha.png):\n")
		report.WriteString(fmt.Sprintf("  Dimensions: %dx%d\n", waterAlpha.Bounds().Dx(), waterAlpha.Bounds().Dy()))
		report.WriteString(fmt.Sprintf("  Water pixels: %d / %d\n", waterPixels, totalPixels))
		report.WriteString(fmt.Sprintf("  Coverage: %.2f%%\n\n", waterPct))
	}

	report.WriteString("FILES GENERATED:\n")
	report.WriteString("  00_overpass_water_only.json - Water elements from Overpass API (filtered)\n")
	report.WriteString("  01_water_alpha.png - Extracted water alpha mask\n")
	report.WriteString("  00_rendered_water.png - Mapnik-rendered water layer\n")
	report.WriteString("  (see other numbered files for full pipeline stages)\n\n")

	report.WriteString("DEBUGGING TIPS:\n")
	report.WriteString("1. Compare 00_overpass_water_only.json with 01_water_alpha.png\n")
	report.WriteString("2. Check if Overpass water elements match rendered water pixels\n")
	report.WriteString("3. Look at 00_rendered_water.png to verify Mapnik rendering\n")
	report.WriteString("4. Check feature counts - missing features might indicate extraction issues\n")
	report.WriteString("5. The JSON file contains only water elements to reduce file size\n")

	return report.String()
}

// countNonZeroPixels counts pixels with non-zero alpha/gray values
func countNonZeroPixels(img *image.Gray) int {
	if img == nil {
		return 0
	}
	count := 0
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.GrayAt(x, y).Y > 0 {
				count++
			}
		}
	}
	return count
}

// isWaterElement checks if an OSM element represents water based on its tags
func isWaterElement(tags map[string]string) bool {
	if tags == nil {
		return false
	}

	// Check for natural=water or natural=coastline
	if natural, ok := tags["natural"]; ok {
		if natural == "water" || natural == "coastline" {
			return true
		}
	}

	// Check for waterway tag
	if waterway, ok := tags["waterway"]; ok {
		return waterway != ""
	}

	return false
}

// extractWaterElements creates a filtered copy of Overpass result containing only water elements.
// This significantly reduces file size by excluding non-water ways, relations, and unused nodes.
func extractWaterElements(result *overpass.Result) map[string]interface{} {
	if result == nil {
		return nil
	}

	waterWays := make(map[string]interface{})
	waterRelations := make(map[string]interface{})
	neededNodeIDs := make(map[int64]bool)

	// Extract water ways and collect referenced node IDs
	for id, way := range result.Ways {
		if isWaterElement(way.Tags) {
			waterWays[fmt.Sprintf("%d", id)] = way
			// Collect node IDs used by this way
			for _, node := range way.Nodes {
				if node != nil {
					neededNodeIDs[node.ID] = true
				}
			}
		}
	}

	// Extract water relations and collect referenced member IDs
	for id, relation := range result.Relations {
		if isWaterElement(relation.Tags) {
			waterRelations[fmt.Sprintf("%d", id)] = relation
			// Note: Relations may reference ways or nodes, but for JSON debugging
			// we don't need to deeply traverse all members
		}
	}

	// Extract only the nodes that are actually used by water ways
	waterNodes := make(map[string]interface{})
	for nodeID := range neededNodeIDs {
		if node, ok := result.Nodes[nodeID]; ok {
			waterNodes[fmt.Sprintf("%d", nodeID)] = node
		}
	}

	return map[string]interface{}{
		"timestamp": result.Timestamp,
		"count":     len(waterWays) + len(waterRelations),
		"nodes":     waterNodes,
		"ways":      waterWays,
		"relations": waterRelations,
		"summary": map[string]int{
			"water_ways":      len(waterWays),
			"water_relations": len(waterRelations),
			"water_nodes":     len(waterNodes),
			"total_in_tile":   len(result.Ways) + len(result.Relations),
		},
	}
}
