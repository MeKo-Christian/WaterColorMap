package watercolor

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
)

// LayerStyle defines per-layer watercolor styling parameters.
type LayerStyle struct {
	Layer             geojson.LayerType
	Texture           image.Image
	Tint              color.NRGBA
	TintStrength      float64
	MaskBlurSigma     float32 // optional override for mask blur (0 uses Params.BlurSigma)
	MaskNoiseStrength float64 // optional override for noise strength (<=0 uses Params.NoiseStrength)
	ShadeSigma        float32 // optional additional blur for a dark overlay (0 disables)
	ShadeStrength     float64 // 0 disables
	EdgeColor         color.NRGBA
	EdgeStrength      float64
	EdgeInnerSigma    float32
	EdgeOuterSigma    float32
	EdgeGamma         float64
}

// Params define the common watercolor processing knobs.
type Params struct {
	TileSize       int
	BlurSigma      float32
	NoiseScale     float64
	NoiseStrength  float64
	Threshold      uint8
	AntialiasSigma float32
	Seed           int64
	OffsetX        int
	OffsetY        int
	Styles         map[geojson.LayerType]LayerStyle
}

// DefaultParams returns sensible defaults for the watercolor pipeline.
// textures provides base textures per layer; caller may omit entries for layers they won't process.
func DefaultParams(tileSize int, seed int64, textures map[geojson.LayerType]image.Image) Params {
	return Params{
		TileSize:       tileSize,
		BlurSigma:      2.0,
		NoiseScale:     30.0,
		NoiseStrength:  0.3,
		Threshold:      128,
		AntialiasSigma: 0.5,
		Seed:           seed,
		Styles: map[geojson.LayerType]LayerStyle{
			geojson.LayerLand: {
				Layer:          geojson.LayerLand,
				Texture:        textures[geojson.LayerLand],
				Tint:           color.NRGBA{R: 210, G: 190, B: 160, A: 255},
				TintStrength:   0.25,
				ShadeSigma:     4.5,
				ShadeStrength:  0.12,
				EdgeColor:      color.NRGBA{R: 120, G: 100, B: 80, A: 255},
				EdgeStrength:   0.35,
				EdgeInnerSigma: 1.0,
				EdgeOuterSigma: 3.0,
				EdgeGamma:      1.5,
			},
			geojson.LayerWater: {
				Layer:          geojson.LayerWater,
				Texture:        textures[geojson.LayerWater],
				Tint:           color.NRGBA{R: 140, G: 180, B: 220, A: 255},
				TintStrength:   0.3,
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeColor:      color.NRGBA{R: 70, G: 110, B: 150, A: 255},
				EdgeStrength:   0.45,
				EdgeInnerSigma: 1.0,
				EdgeOuterSigma: 3.5,
				EdgeGamma:      1.3,
			},
			geojson.LayerParks: {
				Layer:          geojson.LayerParks,
				Texture:        textures[geojson.LayerParks],
				Tint:           color.NRGBA{R: 120, G: 170, B: 110, A: 255},
				TintStrength:   0.3,
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeColor:      color.NRGBA{R: 70, G: 120, B: 70, A: 255},
				EdgeStrength:   0.4,
				EdgeInnerSigma: 1.0,
				EdgeOuterSigma: 3.0,
				EdgeGamma:      1.4,
			},
			geojson.LayerRoads: {
				Layer:             geojson.LayerRoads,
				Texture:           textures[geojson.LayerRoads],
				Tint:              color.NRGBA{R: 230, G: 170, B: 110, A: 255},
				TintStrength:      0.35,
				MaskBlurSigma:     1.1,
				MaskNoiseStrength: 0.25,
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeColor:         color.NRGBA{R: 160, G: 100, B: 60, A: 255},
				EdgeStrength:      0.55,
				EdgeInnerSigma:    0.6,
				EdgeOuterSigma:    1.5,
				EdgeGamma:         1.1,
			},
			geojson.LayerHighways: {
				Layer:             geojson.LayerHighways,
				Texture:           textures[geojson.LayerHighways],
				Tint:              color.NRGBA{R: 255, G: 230, B: 140, A: 255},
				TintStrength:      0.15,
				MaskBlurSigma:     0.9,
				MaskNoiseStrength: 0.18,
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeColor:         color.NRGBA{R: 170, G: 140, B: 60, A: 255},
				EdgeStrength:      0.45,
				EdgeInnerSigma:    0.6,
				EdgeOuterSigma:    1.4,
				EdgeGamma:         1.05,
			},
			geojson.LayerCivic: {
				Layer:          geojson.LayerCivic,
				Texture:        textures[geojson.LayerCivic],
				Tint:           color.NRGBA{R: 190, G: 170, B: 190, A: 255},
				TintStrength:   0.2,
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeColor:      color.NRGBA{R: 120, G: 90, B: 130, A: 255},
				EdgeStrength:   0.4,
				EdgeInnerSigma: 1.0,
				EdgeOuterSigma: 2.5,
				EdgeGamma:      1.2,
			},
		},
	}
}

func processMask(baseMask *image.Gray, layer geojson.LayerType, params Params) (*image.Gray, error) {
	if baseMask == nil {
		return nil, errors.New("base mask is nil")
	}
	style, ok := params.Styles[layer]
	if !ok {
		return nil, fmt.Errorf("missing style for layer %s", layer)
	}

	// Blur/noise/threshold/AA pipeline on the provided mask.
	layerBlur := params.BlurSigma
	if style.MaskBlurSigma > 0 {
		layerBlur = style.MaskBlurSigma
	}
	layerNoiseStrength := params.NoiseStrength
	if style.MaskNoiseStrength > 0 {
		layerNoiseStrength = style.MaskNoiseStrength
	}

	blurred := mask.GaussianBlur(baseMask, layerBlur)
	noise := mask.GeneratePerlinNoiseWithOffset(params.TileSize, params.TileSize, params.NoiseScale, params.Seed, params.OffsetX, params.OffsetY)
	noisy := blurred
	if layerNoiseStrength != 0 {
		noisy = mask.ApplyNoiseToMask(blurred, noise, layerNoiseStrength)
	}
	thresholded := mask.ApplyThreshold(noisy, params.Threshold)
	finalMask := thresholded
	if params.AntialiasSigma > 0 {
		finalMask = mask.AntialiasEdges(thresholded, params.AntialiasSigma)
	}

	return finalMask, nil
}

func paintFromFinalMask(finalMask *image.Gray, layer geojson.LayerType, params Params) (*image.NRGBA, error) {
	style, ok := params.Styles[layer]
	if !ok {
		return nil, fmt.Errorf("missing style for layer %s", layer)
	}
	if params.TileSize <= 0 {
		return nil, errors.New("tile size must be positive")
	}
	if style.Texture == nil {
		return nil, fmt.Errorf("texture is nil for layer %s", layer)
	}
	if finalMask == nil {
		return nil, errors.New("final mask is nil")
	}

	// Texture + mask.
	tiled := texture.TileTexture(style.Texture, params.TileSize, params.OffsetX, params.OffsetY)
	tinted := texture.TintTexture(tiled, style.Tint, style.TintStrength)
	painted := texture.ApplyMaskToTexture(tinted, finalMask)

	// Optional additional shading: blur the final mask further and apply a subtle black overlay.
	result := painted
	if style.ShadeSigma > 0 && style.ShadeStrength > 0 {
		shade := mask.GaussianBlur(finalMask, style.ShadeSigma)
		result = mask.ApplyEdgeDarkening(result, shade, color.NRGBA{R: 0, G: 0, B: 0, A: 255}, style.ShadeStrength)
	}

	// Edge darkening.
	edgeMask := mask.CreateEdgeMask(finalMask, style.EdgeInnerSigma, style.EdgeOuterSigma)
	if edgeMask == nil {
		return nil, errors.New("failed to create edge mask")
	}
	if style.EdgeGamma != 1.0 {
		edgeMask = mask.TaperEdgeMask(edgeMask, style.EdgeGamma)
	}
	result = mask.ApplyEdgeDarkening(result, edgeMask, style.EdgeColor, style.EdgeStrength)

	return result, nil
}

// PaintLayer applies the watercolor pipeline to a single rendered layer image.
func PaintLayer(layerImage image.Image, layer geojson.LayerType, params Params) (*image.NRGBA, error) {
	style, ok := params.Styles[layer]
	if !ok {
		return nil, fmt.Errorf("missing style for layer %s", layer)
	}
	if params.TileSize <= 0 {
		return nil, errors.New("tile size must be positive")
	}
	if style.Texture == nil {
		return nil, fmt.Errorf("texture is nil for layer %s", layer)
	}
	if params.NoiseScale <= 0 {
		return nil, errors.New("noise scale must be positive")
	}

	// Use alpha-only mask as the base input for the mask pipeline.
	baseMask := mask.ExtractAlphaMask(layerImage)
	finalMask, err := processMask(baseMask, layer, params)
	if err != nil {
		return nil, err
	}
	return paintFromFinalMask(finalMask, layer, params)
}

// PaintLayerFromMask runs the mask pipeline (blur/noise/threshold/AA) on a provided alpha mask,
// then applies texture/tinting and edge/shading. This is used for cross-layer workflows.
func PaintLayerFromMask(baseMask *image.Gray, layer geojson.LayerType, params Params) (*image.NRGBA, error) {
	if params.NoiseScale <= 0 {
		return nil, errors.New("noise scale must be positive")
	}
	finalMask, err := processMask(baseMask, layer, params)
	if err != nil {
		return nil, err
	}
	return paintFromFinalMask(finalMask, layer, params)
}

// PaintLayerFromFinalMask skips the blur/noise/threshold steps and paints directly from a final mask.
// Useful when the final mask is derived from other layers (e.g. landMask = invert(nonLandMask)).
func PaintLayerFromFinalMask(finalMask *image.Gray, layer geojson.LayerType, params Params) (*image.NRGBA, error) {
	return paintFromFinalMask(finalMask, layer, params)
}
