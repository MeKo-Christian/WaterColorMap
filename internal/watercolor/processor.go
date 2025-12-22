package watercolor

import (
	"errors"
	"fmt"
	"image"

	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/mask"
	"github.com/MeKo-Tech/watercolormap/internal/texture"
)

// LayerStyle defines per-layer watercolor styling parameters.
type LayerStyle struct {
	Texture           image.Image
	Layer             geojson.LayerType
	EdgeStrength      float64
	MaskNoiseStrength float64
	ShadeStrength     float64
	EdgeGamma         float64
	MaskBlurSigma     float32
	ShadeSigma        float32
	EdgeSigma    float32
	MaskThreshold     *uint8 // Optional per-layer threshold override (if nil, uses global Params.Threshold)
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
	PerlinNoise    *image.Gray // Pre-generated noise texture, reused across all layers to avoid redundant allocations
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

// ptr is a helper to create uint8 pointers for optional threshold values.
func ptr(v uint8) *uint8 { return &v }

// DefaultParams returns sensible defaults for the watercolor pipeline.
// textures provides base textures per layer; caller may omit entries for layers they won't process.
func DefaultParams(tileSize int, seed int64, textures map[geojson.LayerType]image.Image) Params {
	return Params{
		TileSize:       tileSize,
		BlurSigma:      1.2,
		NoiseScale:     30.0,
		NoiseStrength:  0.28,
		Threshold:      50,
		AntialiasSigma: 0.5,
		Seed:           seed,
		Styles: map[geojson.LayerType]LayerStyle{
			geojson.LayerLand: {
				Layer:          geojson.LayerLand,
				Texture:        textures[geojson.LayerLand],
				ShadeSigma:     3.5,
				ShadeStrength:  0.12,
				EdgeStrength:   0.2,
				EdgeSigma: 3.0,
				EdgeGamma:      1.5,
			},
			geojson.LayerWater: {
				Layer:          geojson.LayerWater,
				Texture:        textures[geojson.LayerWater],
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeStrength:   0.2,
				EdgeSigma: 3.5,
				EdgeGamma:      1.4,
			},
			geojson.LayerRivers: {
				Layer:             geojson.LayerRivers,
				Texture:           textures[geojson.LayerWater], // Use same texture as water
				MaskBlurSigma:     0.8,                          // Light blur for natural river edges
				MaskNoiseStrength: 0.15,                         // Subtle noise for organic feel
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeStrength:      0.2,
				EdgeSigma:    2.5,
				EdgeGamma:         1.3,
			},
			geojson.LayerParks: {
				Layer:          geojson.LayerParks,
				Texture:        textures[geojson.LayerParks],
				MaskThreshold:  ptr(120), // Higher threshold for layers after land
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeStrength:   0.2,
				EdgeSigma: 3.0,
				EdgeGamma:      1.4,
			},
			geojson.LayerRoads: {
				Layer:             geojson.LayerRoads,
				Texture:           textures[geojson.LayerRoads],
				MaskThreshold:     ptr(120), // Higher threshold for layers after land
				MaskBlurSigma:     1.4,
				MaskNoiseStrength: 0.25,
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeStrength:      0.2,
				EdgeSigma:    1.5,
				EdgeGamma:         1.6,
			},
			geojson.LayerHighways: {
				Layer:             geojson.LayerHighways,
				Texture:           textures[geojson.LayerHighways],
				MaskThreshold:     ptr(120), // Higher threshold for layers after land
				MaskBlurSigma:     1.1,
				MaskNoiseStrength: 0.18,
				ShadeSigma:        0,
				ShadeStrength:     0,
				EdgeStrength:      0.2,
				EdgeSigma:    1.4,
				EdgeGamma:         1.5,
			},
			geojson.LayerCivic: {
				Layer:          geojson.LayerCivic,
				Texture:        textures[geojson.LayerCivic],
				MaskThreshold:  ptr(120), // Higher threshold for layers after land
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeStrength:   0.2,
				EdgeSigma: 2.5,
				EdgeGamma:      1.4,
			},
			geojson.LayerBuildings: {
				Layer:          geojson.LayerBuildings,
				Texture:        textures[geojson.LayerCivic], // Use same texture as civic
				MaskThreshold:  ptr(120),                     // Higher threshold for layers after land
				ShadeSigma:     0,
				ShadeStrength:  0,
				EdgeStrength:   0.2,
				EdgeSigma: 2.5,
				EdgeGamma:      1.5,
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

	// Use per-layer threshold if specified, otherwise use global threshold
	threshold := params.Threshold
	if style.MaskThreshold != nil {
		threshold = *style.MaskThreshold
	}

	blurred := mask.BoxBlurSigma(baseMask, layerBlur)
	noisy := blurred
	if layerNoiseStrength != 0 {
		noisy = mask.ApplyNoiseToMask(blurred, params.PerlinNoise, layerNoiseStrength)
	}
	finalMask := mask.ApplyThresholdWithAntialias(noisy, threshold)

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
	painted := texture.ApplyMaskToTexture(tiled, finalMask)

	// Optional additional shading: blur the final mask further and apply a subtle darkening.
	result := painted
	if style.ShadeSigma > 0 && style.ShadeStrength > 0 {
		shade := mask.BoxBlurSigma(finalMask, style.ShadeSigma)
		// Invert shade mask: we want to darken where the feature IS (high values in finalMask)
		// ApplySoftEdgeMask expects 255=no change, 0=darken, so invert the blurred mask
		invertedShade := mask.InvertMask(shade)
		result = mask.ApplySoftEdgeMask(result, invertedShade, style.ShadeStrength)
	}

	// Edge darkening using distance-based edge mask
	// Convert sigma parameters to radius (approximation: radius ≈ 3*sigma)
	radius := float64(style.EdgeSigma * 3.0)
	gamma := style.EdgeGamma
	if gamma <= 0 {
		gamma = 1.0
	}

	edgeMask := mask.CreateDistanceEdgeMask(finalMask, radius, gamma)
	if edgeMask == nil {
		return nil, errors.New("failed to create edge mask")
	}
	// ApplySoftEdgeMask expects: 255=no change, 0=maximum effect
	// CreateDistanceEdgeMask produces: 255=no effect (center), 0=max effect (edges)
	result = mask.ApplySoftEdgeMask(result, edgeMask, style.EdgeStrength)

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
