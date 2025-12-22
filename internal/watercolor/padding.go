package watercolor

import "math"

// MinGeometryPaddingPx is the minimum padding needed to accommodate polygon
// geometry that extends beyond tile boundaries. OSM polygons (water bodies,
// parks, etc.) often extend significantly beyond the tile they intersect.
// A larger padding ensures these polygons render correctly without straight-line
// clipping artifacts at tile edges.
const MinGeometryPaddingPx = 64

// RequiredPaddingPx returns a suggested pixel padding for "metatile" rendering.
//
// The watercolor pipeline applies multiple Gaussian blurs (mask blur, antialias,
// edge halo, optional shading). Those filters need valid pixels outside the
// final tile area to avoid boundary artifacts. Rendering and processing a larger
// tile (tileSize + 2*pad) and cropping back to the center removes seams.
//
// Additionally, polygon geometry that crosses tile boundaries needs extra space
// to render correctly. The returned padding is the maximum of blur requirements
// and geometry requirements (MinGeometryPaddingPx).
func RequiredPaddingPx(params Params) int {
	maxSigma := float32(0)

	consider := func(s float32) {
		if s > maxSigma {
			maxSigma = s
		}
	}

	consider(params.BlurSigma)
	consider(params.AntialiasSigma)

	for _, style := range params.Styles {
		consider(style.MaskBlurSigma)
		consider(style.ShadeSigma)
		consider(style.EdgeSigma)
	}

	// 3*sigma captures the vast majority of the kernel energy.
	blurPad := int(math.Ceil(float64(maxSigma)*3.0)) + 2
	if blurPad < 1 {
		blurPad = 1
	}

	// Use the larger of blur padding and geometry padding
	if blurPad < MinGeometryPaddingPx {
		return MinGeometryPaddingPx
	}
	return blurPad
}
