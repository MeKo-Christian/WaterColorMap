package watercolor

import (
	"image"
	"image/color"
	"testing"

	"github.com/MeKo-Tech/watercolormap/internal/geojson"
)

func solidTexture(w, h int, c color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func TestPaintLayerAppliesMaskTintAndEdge(t *testing.T) {
	tileSize := 16
	layer := geojson.LayerWater

	baseColor := color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	textures := map[geojson.LayerType]image.Image{
		layer: solidTexture(4, 4, baseColor),
	}

	params := DefaultParams(tileSize, 123, textures)
	params.NoiseStrength = 0.0  // deterministic
	params.AntialiasSigma = 0.0 // keep crisp for assertions
	params.BlurSigma = 1.0      // mild blur for edge halo
	params.Threshold = 128      // retain shape
	params.OffsetX = 0
	params.OffsetY = 0

	style := params.Styles[layer]
	style.Tint = color.NRGBA{R: 200, G: 50, B: 50, A: 255}
	style.TintStrength = 0.4
	style.EdgeColor = color.NRGBA{R: 60, G: 40, B: 30, A: 255}
	style.EdgeStrength = 0.5
	style.EdgeInnerSigma = 0.5
	style.EdgeOuterSigma = 2.0
	style.EdgeGamma = 1.0
	params.Styles[layer] = style

	// Build a simple square feature mask
	layerImg := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			layerImg.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}

	out, err := PaintLayer(layerImg, layer, params)
	if err != nil {
		t.Fatalf("PaintLayer returned error: %v", err)
	}
	if out == nil {
		t.Fatal("PaintLayer returned nil output")
	}

	// Outside feature should be fully transparent
	if got := out.NRGBAAt(0, 0); got.A != 0 {
		t.Fatalf("expected outside alpha 0, got %d", got.A)
	}

	// Center should be tinted (may be slightly darkened depending on halo spread)
	center := out.NRGBAAt(8, 8)
	expectedTint := color.NRGBA{
		R: uint8(0.6*float64(baseColor.R) + 0.4*float64(style.Tint.R)),
		G: uint8(0.6*float64(baseColor.G) + 0.4*float64(style.Tint.G)),
		B: uint8(0.6*float64(baseColor.B) + 0.4*float64(style.Tint.B)),
		A: 255,
	}
	if (center.R > expectedTint.R || center.G > expectedTint.G || center.B > expectedTint.B) || center.A != 255 {
		t.Fatalf("unexpected center color %+v, expected tinted (<= %+v) with alpha 255", center, expectedTint)
	}

	// Edge region should include darker pixels than center due to halo darkening
	darkest := center
	for y := 0; y < tileSize; y++ {
		for x := 0; x < tileSize; x++ {
			p := out.NRGBAAt(x, y)
			if p.A == 255 && p.R < darkest.R {
				darkest = p
			}
		}
	}
	if darkest.R >= center.R && darkest.G >= center.G && darkest.B >= center.B {
		t.Fatalf("expected some pixels to be darkened relative to center; center=%+v darkest=%+v", center, darkest)
	}
}

func TestPaintLayerMissingStyle(t *testing.T) {
	params := Params{
		TileSize:   16,
		NoiseScale: 10,
		Styles:     map[geojson.LayerType]LayerStyle{},
	}
	_, err := PaintLayer(image.NewRGBA(image.Rect(0, 0, 4, 4)), geojson.LayerLand, params)
	if err == nil {
		t.Fatal("expected error for missing style")
	}
}
