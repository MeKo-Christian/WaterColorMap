package watercolor

import "math"

// RequiredPaddingPx returns a suggested pixel padding for "metatile" rendering.
//
// The watercolor pipeline applies multiple Gaussian blurs (mask blur, antialias,
// edge halo, optional shading). Those filters need valid pixels outside the
// final tile area to avoid boundary artifacts. Rendering and processing a larger
// tile (tileSize + 2*pad) and cropping back to the center removes seams.
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
		consider(style.EdgeInnerSigma)
		consider(style.EdgeOuterSigma)
	}

	if maxSigma <= 0 {
		return 0
	}

	// 3*sigma captures the vast majority of the kernel energy.
	pad := int(math.Ceil(float64(maxSigma)*3.0)) + 2
	if pad < 1 {
		pad = 1
	}
	return pad
}
