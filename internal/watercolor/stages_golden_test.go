package watercolor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MeKo-Tech/watercolormap/internal/composite"
	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
)

func TestWatercolorStagesGolden(t *testing.T) {
	// This test is intentionally I/O heavy-ish but deterministic.
	// It exists to visualize the pipeline stages and provide a golden baseline.
	//
	// Update goldens:
	//   UPDATE_GOLDEN=1 go test ./... -run TestWatercolorStagesGolden
	//
	// When goldens don't match, the test writes debug outputs under:
	//   testdata/output/watercolor-stages/<case>/

	// NOTE: go test runs with the working directory set to the package folder
	// (internal/watercolor). We want goldens/debug outputs in the repo root.
	goldenDir := filepath.Join("..", "..", "testdata", "golden", "watercolor-stages")
	debugRoot := filepath.Join("..", "..", "testdata", "output", "watercolor-stages")
	caseName := "stamen-nonland-landmask"
	debugDir := filepath.Join(debugRoot, caseName)

	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("failed to create golden dir: %v", err)
	}
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		t.Fatalf("failed to create debug dir: %v", err)
	}

	update := os.Getenv("UPDATE_GOLDEN") == "1"

	// --- Build simple, deterministic synthetic layers ---
	// We keep this in-code so the test is self-contained.
	// Shapes are chosen to make each stage clearly visible.
	tileSize := 128
	seed := int64(123)

	waterLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	roadsLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	highwaysLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	parksLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	civicLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))

	// water: bottom half
	for y := tileSize / 2; y < tileSize; y++ {
		for x := 0; x < tileSize; x++ {
			waterLayer.SetNRGBA(x, y, color.NRGBA{R: 10, G: 40, B: 200, A: 255})
		}
	}

	// roads: a diagonal band crossing both halves
	for y := 0; y < tileSize; y++ {
		for x := 0; x < tileSize; x++ {
			if x-y >= -2 && x-y <= 2 {
				roadsLayer.SetNRGBA(x, y, color.NRGBA{R: 255, G: 200, B: 0, A: 255})
			}
		}
	}

	// highways: a thinner diagonal inside the roads band
	for y := 0; y < tileSize; y++ {
		for x := 0; x < tileSize; x++ {
			if x-y >= -1 && x-y <= 1 {
				highwaysLayer.SetNRGBA(x, y, color.NRGBA{R: 255, G: 230, B: 0, A: 255})
			}
		}
	}

	// parks: a square mostly on land but overlapping the diagonal road
	for y := 16; y < 48; y++ {
		for x := 24; x < 64; x++ {
			parksLayer.SetNRGBA(x, y, color.NRGBA{R: 0, G: 255, B: 0, A: 255})
		}
	}

	// civic: a square partially in water, to verify land constraint
	for y := 56; y < 88; y++ {
		for x := 80; x < 112; x++ {
			civicLayer.SetNRGBA(x, y, color.NRGBA{R: 180, G: 120, B: 180, A: 255})
		}
	}

	textures := map[geojson.LayerType]image.Image{
		geojson.LayerPaper:    solidTexture(8, 8, color.NRGBA{R: 255, G: 255, B: 255, A: 255}),
		geojson.LayerLand:     solidTexture(8, 8, color.NRGBA{R: 200, G: 190, B: 175, A: 255}),
		geojson.LayerWater:    solidTexture(8, 8, color.NRGBA{R: 120, G: 150, B: 200, A: 255}),
		geojson.LayerParks:    solidTexture(8, 8, color.NRGBA{R: 140, G: 180, B: 140, A: 255}),
		geojson.LayerCivic:    solidTexture(8, 8, color.NRGBA{R: 190, G: 170, B: 190, A: 255}),
		geojson.LayerRoads:    solidTexture(8, 8, color.NRGBA{R: 255, G: 255, B: 255, A: 255}),
		geojson.LayerHighways: solidTexture(8, 8, color.NRGBA{R: 255, G: 230, B: 120, A: 255}),
	}

	params := DefaultParams(tileSize, seed, textures)
	params.OffsetX = 0
	params.OffsetY = 0
	params.NoiseScale = 30
	params.NoiseStrength = 0.35
	params.BlurSigma = 2.0
	params.Threshold = 128
	params.AntialiasSigma = 0.6

	// --- Build Stamen-aligned masks (alpha-only) ---
	waterAlpha := mask.ExtractAlphaMask(waterLayer)
	roadsAlpha := mask.ExtractAlphaMask(roadsLayer)
	highwaysAlpha := mask.ExtractAlphaMask(highwaysLayer)
	nonLandBase := mask.MaxMask(waterAlpha, roadsAlpha)

	blur1 := mask.GaussianBlur(nonLandBase, params.BlurSigma)
	noise := mask.GeneratePerlinNoiseWithOffset(tileSize, tileSize, params.NoiseScale, params.Seed, params.OffsetX, params.OffsetY)
	noisy := mask.ApplyNoiseToMask(blur1, noise, params.NoiseStrength)
	thresholded := mask.ApplyThreshold(noisy, params.Threshold)
	aa := mask.AntialiasEdges(thresholded, params.AntialiasSigma)
	landMask := mask.InvertMask(aa)

	parksAlpha := mask.ExtractAlphaMask(parksLayer)
	civicAlpha := mask.ExtractAlphaMask(civicLayer)
	parksOnLand := mask.MinMask(parksAlpha, landMask)
	civicOnLand := mask.MinMask(civicAlpha, landMask)

	// --- Painted outputs (final view per layer) ---
	paintedWater, err := PaintLayer(waterLayer, geojson.LayerWater, params)
	if err != nil {
		t.Fatalf("PaintLayer(water) failed: %v", err)
	}
	paintedHighways, err := PaintLayer(highwaysLayer, geojson.LayerHighways, params)
	if err != nil {
		t.Fatalf("PaintLayer(highways) failed: %v", err)
	}
	paintedLand, err := PaintLayerFromFinalMask(landMask, geojson.LayerLand, params)
	if err != nil {
		t.Fatalf("PaintLayerFromFinalMask(land) failed: %v", err)
	}
	paintedParks, err := PaintLayerFromMask(parksOnLand, geojson.LayerParks, params)
	if err != nil {
		t.Fatalf("PaintLayerFromMask(parks) failed: %v", err)
	}
	paintedCivic, err := PaintLayerFromMask(civicOnLand, geojson.LayerCivic, params)
	if err != nil {
		t.Fatalf("PaintLayerFromMask(civic) failed: %v", err)
	}

	base := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	paper := textures[geojson.LayerPaper]
	if paper != nil {
		base = texture.TileTexture(paper, tileSize, 0, 0)
	}
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
		t.Fatalf("CompositeLayers failed: %v", err)
	}

	stages := map[string]image.Image{
		"01_water_alpha.png":      waterAlpha,
		"02_roads_alpha.png":      roadsAlpha,
		"03_highways_alpha.png":   highwaysAlpha,
		"04_nonland_union.png":    nonLandBase,
		"04_blur.png":             blur1,
		"05_noise.png":            noise,
		"06_noisy.png":            noisy,
		"07_threshold.png":        thresholded,
		"08_antialias.png":        aa,
		"09_land_inverted.png":    landMask,
		"10_parks_on_land.png":    parksOnLand,
		"11_civic_on_land.png":    civicOnLand,
		"12_painted_water.png":    paintedWater,
		"13_painted_land.png":     paintedLand,
		"14_painted_parks.png":    paintedParks,
		"15_painted_civic.png":    paintedCivic,
		"16_painted_highways.png": paintedHighways,
		"17_combined.png":         combined,
	}

	keys := make([]string, 0, len(stages))
	for k := range stages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Always write debug outputs for this run (stable names, overwritten).
	for _, name := range keys {
		img := stages[name]
		if img == nil {
			t.Fatalf("stage %s is nil", name)
		}
		if err := writePNG(filepath.Join(debugDir, name), img); err != nil {
			t.Fatalf("failed to write debug png %s: %v", name, err)
		}
	}

	// Update or compare goldens.
	for _, name := range keys {
		img := stages[name]
		goldenPath := filepath.Join(goldenDir, name)

		if update {
			if err := writePNG(goldenPath, img); err != nil {
				t.Fatalf("failed to update golden %s: %v", name, err)
			}
			continue
		}

		exists, err := fileExists(goldenPath)
		if err != nil {
			t.Fatalf("failed to stat golden %s: %v", goldenPath, err)
		}
		if !exists {
			t.Fatalf("missing golden %s; run: UPDATE_GOLDEN=1 go test ./... -run TestWatercolorStagesGolden", goldenPath)
		}

		gotBytes, err := encodePNG(img)
		if err != nil {
			t.Fatalf("failed to encode generated %s: %v", name, err)
		}
		wantBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("failed to read golden %s: %v", goldenPath, err)
		}

		// Compare decoded pixels, not raw bytes (PNG encoding may vary).
		gotImg, err := png.Decode(bytes.NewReader(gotBytes))
		if err != nil {
			t.Fatalf("failed to decode generated png %s: %v", name, err)
		}
		wantImg, err := png.Decode(bytes.NewReader(wantBytes))
		if err != nil {
			t.Fatalf("failed to decode golden png %s: %v", name, err)
		}

		if !imagesEqual(gotImg, wantImg) {
			t.Fatalf("golden mismatch for %s\n- golden: %s\n- got: %s\nTo update: UPDATE_GOLDEN=1 go test ./... -run TestWatercolorStagesGolden",
				name,
				goldenPath,
				filepath.Join(debugDir, name),
			)
		}
	}

	if update {
		t.Logf("updated goldens in %s (debug outputs in %s)", goldenDir, debugDir)
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func writePNG(path string, img image.Image) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func imagesEqual(a, b image.Image) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Bounds() != b.Bounds() {
		return false
	}
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca := color.NRGBAModel.Convert(a.At(x, y)).(color.NRGBA)
			cb := color.NRGBAModel.Convert(b.At(x, y)).(color.NRGBA)
			if ca != cb {
				return false
			}
		}
	}
	return true
}
