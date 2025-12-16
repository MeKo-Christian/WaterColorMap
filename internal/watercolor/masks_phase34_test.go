package watercolor

import (
	"image"
	"image/color"
	"testing"

	"github.com/MeKo-Tech/watercolormap/internal/mask"
)

func TestDerivedLandMaskExcludesWaterAndRoads(t *testing.T) {
	// This test asserts the Phase 3.4 invariant:
	// landMask := invert(process(max(waterAlpha, roadsAlpha)))
	// => land is fully excluded where water/roads are present.
	//
	// We keep noise disabled here to make the result strictly deterministic.
	tileSize := 64

	waterLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	roadsLayer := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))

	// water: bottom half fully opaque
	for y := tileSize / 2; y < tileSize; y++ {
		for x := 0; x < tileSize; x++ {
			waterLayer.SetNRGBA(x, y, color.NRGBA{R: 10, G: 40, B: 200, A: 255})
		}
	}

	// roads: vertical band through the whole tile
	for y := 0; y < tileSize; y++ {
		for x := 28; x <= 35; x++ {
			roadsLayer.SetNRGBA(x, y, color.NRGBA{R: 255, G: 200, B: 0, A: 255})
		}
	}

	params := Params{
		TileSize:       tileSize,
		BlurSigma:      2.0,
		NoiseScale:     30.0,
		NoiseStrength:  0.0,
		Threshold:      128,
		AntialiasSigma: 0.0,
		Seed:           123,
		OffsetX:        0,
		OffsetY:        0,
	}

	waterAlpha := mask.ExtractAlphaMask(waterLayer)
	roadsAlpha := mask.ExtractAlphaMask(roadsLayer)
	nonLandBase := mask.MaxMask(waterAlpha, roadsAlpha)

	blurred := mask.GaussianBlur(nonLandBase, params.BlurSigma)
	thresholded := mask.ApplyThreshold(blurred, params.Threshold)
	landMask := mask.InvertMask(thresholded)

	// Deep inside water => land must be excluded.
	if got := landMask.GrayAt(5, tileSize-5).Y; got > 5 {
		t.Fatalf("expected land excluded inside water, got %d", got)
	}
	// Deep inside road band => land must be excluded.
	if got := landMask.GrayAt(32, 5).Y; got > 5 {
		t.Fatalf("expected land excluded inside roads, got %d", got)
	}
	// Far away from water and roads => land must be present.
	if got := landMask.GrayAt(5, 5).Y; got < 250 {
		t.Fatalf("expected land present away from non-land, got %d", got)
	}
}
