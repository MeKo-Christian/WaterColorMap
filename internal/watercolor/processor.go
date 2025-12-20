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
	Texture           image.Image
	Layer             geojson.LayerType
	EdgeStrength      float64
	TintStrength      float64
	MaskNoiseStrength float64
	ShadeStrength     float64
	EdgeGamma         float64
	MaskBlurSigma     float32
	ShadeSigma        float32
	EdgeInnerSigma    float32
	EdgeOuterSigma    float32
	EdgeColor         color.NRGBA
	Tint              color.NRGBA
}

// Params define the common watercolor processing knobs.
type Params struct {
	Styles         map[geojson.LayerType]LayerStyle
	TileSize       int
	NoiseScale     float64
	NoiseStrength  float64
	Seed           int64
	OffsetX        int
	OffsetY        int
	BlurSigma      float32
	AntialiasSigma float32
	Threshold      uint8
}

// ZoomAdjustedBlurSigma returns blur sigma adjusted for zoom level.
// Higher zoom levels (more detail) get sharper edges (less blur).
// baseBlurSigma is the blur at zoom 13; sigma decreases at higher zooms.
func ZoomAdjustedBlurSigma(baseBlurSigma float32, zoom int) float32 {
	// At zoom 10-11: use base * 1.4 (softer for overview)
	// At zoom 12-13: use base (reference level)
	// At zoom 14+: use base * 0.7 (sharper for detail)
	if zoom <= 11 {
		return baseBlurSigma * 1.4
	} else if zoom >= 14 {
		return baseBlurSigma * 0.7
	}
	return baseBlurSigma
}

// DefaultParams returns sensible defaults for the watercolor pipeline.
// textures provides base textures per layer; caller may omit entries for layers they won't process.
func DefaultParams(tileSize int, seed int64, textures map[geojson.LayerType]image.Image) Params {
	return Params{
		TileSize:       tileSize,
		BlurSigma:      1.2,  // Reduced from 1.8 for sharper edges
		NoiseScale:     30.0,
		NoiseStrength:  0.28,
		Threshold:      128,
		AntialiasSigma: 0.5, // Reduced from 0.6 for crisper edges
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
				Tint:              color.NRGBA{R: 245, G: 180, B: 100, A: 255}, // More saturated orange
				TintStrength:      0.55,                                        // Increased from 0.35 for bolder color
				MaskBlurSigma:     1.4,
				MaskNoiseStrength: 0.25,
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeColor:         color.NRGBA{R: 180, G: 110, B: 50, A: 255}, // Darker edge for definition
				EdgeStrength:      0.65,                                       // Increased from 0.55
				EdgeInnerSigma:    0.6,
				EdgeOuterSigma:    1.5,
				EdgeGamma:         1.1,
			},
			geojson.LayerHighways: {
				Layer:             geojson.LayerHighways,
				Texture:           textures[geojson.LayerHighways],
				Tint:              color.NRGBA{R: 255, G: 200, B: 80, A: 255}, // More saturated yellow-orange
				TintStrength:      0.50,                                       // Increased from 0.15 for much bolder color
				MaskBlurSigma:     1.1,
				MaskNoiseStrength: 0.18,
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeColor:         color.NRGBA{R: 200, G: 140, B: 40, A: 255}, // Stronger edge color
				EdgeStrength:      0.60,                                       // Increased from 0.45
				EdgeInnerSigma:    0.6,
				EdgeOuterSigma:    1.4,
				EdgeGamma:         1.05,
			},
			geojson.LayerCivic: {
				Layer:          geojson.LayerCivic,
				Texture:        textures[geojson.LayerCivic],
				Tint:           color.NRGBA{R: 200, G: 180, B: 200, A: 255}, // Lighter lavender for civic areas
				TintStrength:   0.18,                                        // Subtle tint
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeColor:      color.NRGBA{R: 140, G: 110, B: 150, A: 255},
				EdgeStrength:   0.35,
				EdgeInnerSigma: 1.0,
				EdgeOuterSigma: 2.5,
				EdgeGamma:      1.2,
			},
			geojson.LayerBuildings: {
				Layer:          geojson.LayerBuildings,
				Texture:        textures[geojson.LayerCivic], // Use same texture as civic
				Tint:           color.NRGBA{R: 170, G: 140, B: 180, A: 255}, // Darker lavender for buildings
				TintStrength:   0.35,                                        // Stronger tint for more contrast
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeColor:      color.NRGBA{R: 100, G: 70, B: 120, A: 255}, // Darker edges
				EdgeStrength:   0.50,                                        // Stronger edges
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
