package watercolor

import (
	"image"
	"testing"

	"github.com/MeKo-Tech/watercolormap/internal/geojson"
)

func TestRequiredPaddingPx(t *testing.T) {
	textures := map[geojson.LayerType]image.Image{
		geojson.LayerLand:     image.NewNRGBA(image.Rect(0, 0, 2, 2)),
		geojson.LayerWater:    image.NewNRGBA(image.Rect(0, 0, 2, 2)),
		geojson.LayerParks:    image.NewNRGBA(image.Rect(0, 0, 2, 2)),
		geojson.LayerCivic:    image.NewNRGBA(image.Rect(0, 0, 2, 2)),
		geojson.LayerRoads:    image.NewNRGBA(image.Rect(0, 0, 2, 2)),
		geojson.LayerHighways: image.NewNRGBA(image.Rect(0, 0, 2, 2)),
	}

	params := DefaultParams(256, 123, textures)
	pad := RequiredPaddingPx(params)
	if pad <= 0 {
		t.Fatalf("expected pad > 0 for default params, got %d", pad)
	}

	params.BlurSigma = 0
	params.AntialiasSigma = 0
	for k, s := range params.Styles {
		s.MaskBlurSigma = 0
		s.ShadeSigma = 0
		s.EdgeInnerSigma = 0
		s.EdgeOuterSigma = 0
		params.Styles[k] = s
	}
	if got := RequiredPaddingPx(params); got != 0 {
		t.Fatalf("expected pad 0 when all sigmas are 0, got %d", got)
	}
}
