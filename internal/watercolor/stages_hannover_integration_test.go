package watercolor

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

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

	ds := datasource.NewOverpassDataSource("")
	defer ds.Close()

	for _, coords := range coordsCases {
		coords := coords
		caseName := coords.String()
		debugDir := filepath.Join(debugRoot, caseName)
		if err := os.MkdirAll(debugDir, 0o755); err != nil {
			t.Fatalf("failed to create debug dir: %v", err)
		}

		tileData, err := func() (*types.TileData, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			return ds.FetchTileData(ctx, types.TileCoordinate{Zoom: int(coords.Z), X: int(coords.X), Y: int(coords.Y)})
		}()
		if err != nil {
			t.Fatalf("failed to fetch tile data for %s: %v", caseName, err)
		}

		renderDir := filepath.Join(debugDir, "rendered")
		if err := os.MkdirAll(renderDir, 0o755); err != nil {
			t.Fatalf("failed to create render debug dir: %v", err)
		}

		mpRenderer, err := renderer.NewMultiPassRenderer(stylesDir, renderDir, tileSize)
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

		params := DefaultParams(tileSize, seed, textures)
		params.OffsetX = int(coords.X) * tileSize
		params.OffsetY = int(coords.Y) * tileSize

		baseBounds := image.Rect(0, 0, tileSize, tileSize)
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
		noise := mask.GeneratePerlinNoiseWithOffset(tileSize, tileSize, params.NoiseScale, params.Seed, params.OffsetX, params.OffsetY)
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

		base := texture.TileTexture(textures[geojson.LayerPaper], tileSize, params.OffsetX, params.OffsetY)
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
			tileSize,
		)
		if err != nil {
			t.Fatalf("CompositeLayers failed (%s): %v", caseName, err)
		}

		stages := map[string]image.Image{
			"00_rendered_water.png":    waterImg,
			"00_rendered_roads.png":    roadsImg,
			"00_rendered_highways.png": highwaysImg,
			"00_rendered_parks.png":    parksImg,
			"00_rendered_civic.png":    civicImg,
			"01_water_alpha.png":       waterAlpha,
			"02_roads_alpha.png":       roadsAlpha,
			"03_highways_alpha.png":    highwaysAlpha,
			"04_nonland_union.png":     nonLandBase,
			"04_blur.png":              blur1,
			"05_noise.png":             noise,
			"06_noisy.png":             noisy,
			"07_threshold.png":         thresholded,
			"08_antialias.png":         aa,
			"09_land_inverted.png":     landMask,
			"10_parks_on_land.png":     parksOnLand,
			"11_civic_on_land.png":     civicOnLand,
			"12_painted_water.png":     paintedWater,
			"13_painted_land.png":      paintedLand,
			"14_painted_parks.png":     paintedParks,
			"15_painted_civic.png":     paintedCivic,
			"16_painted_highways.png":  paintedHighways,
			"17_combined.png":          combined,
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
