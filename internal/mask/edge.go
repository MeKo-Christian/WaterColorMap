package mask

import (
	"image"
	"image/color"
	"math"
)

// CreateEdgeMask builds a halo mask around feature edges by differencing inner/outer blurs.
// The result highlights only the edge transition area (outer minus inner), suitable for darkening.
func CreateEdgeMask(mask *image.Gray, innerSigma, outerSigma float32) *image.Gray {
	if mask == nil {
		return nil
	}

	if innerSigma < 0 {
		innerSigma = 0
	}
	if outerSigma < innerSigma {
		outerSigma = innerSigma
	}

	inner := GaussianBlur(mask, innerSigma)
	outer := GaussianBlur(mask, outerSigma)

	dst := image.NewGray(mask.Bounds())
	maxVal := 0

	// First pass: raw difference and find max for normalization.
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			diff := int(outer.GrayAt(x, y).Y) - int(inner.GrayAt(x, y).Y)
			if diff < 0 {
				diff = -diff // absolute difference to capture halo on both sides of the edge
			}
			if diff > 255 {
				diff = 255
			}
			if diff > maxVal {
				maxVal = diff
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(diff)})
		}
	}

	// Normalize to use the full 0-255 range (if there is any edge signal).
	if maxVal > 0 && maxVal < 255 {
		for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
			for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
				val := int(dst.GrayAt(x, y).Y)
				scaled := int(math.Round(float64(val) * 255.0 / float64(maxVal)))
				if scaled > 255 {
					scaled = 255
				}
				dst.SetGray(x, y, color.Gray{Y: uint8(scaled)})
			}
		}
	}

	return dst
}

// TaperEdgeMask applies a gamma curve to control falloff of the edge halo.
// gamma > 1.0 produces a steeper falloff; gamma < 1.0 makes it softer.
func TaperEdgeMask(edge *image.Gray, gamma float64) *image.Gray {
	if edge == nil {
		return nil
	}
	if gamma <= 0 {
		gamma = 1.0
	}

	dst := image.NewGray(edge.Bounds())
	for y := edge.Bounds().Min.Y; y < edge.Bounds().Max.Y; y++ {
		for x := edge.Bounds().Min.X; x < edge.Bounds().Max.X; x++ {
			v := float64(edge.GrayAt(x, y).Y) / 255.0
			tapered := math.Pow(v, gamma)
			if tapered < 0 {
				tapered = 0
			}
			if tapered > 1 {
				tapered = 1
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(math.Round(tapered * 255))})
		}
	}
	return dst
}

// ApplyEdgeDarkening composites a dark edge overlay onto a painted tile.
// The edge mask controls the overlay alpha; strength scales the effect (0-1).
func ApplyEdgeDarkening(
	base *image.NRGBA,
	edgeMask *image.Gray,
	darken color.NRGBA,
	strength float64,
) *image.NRGBA {
	if base == nil || edgeMask == nil {
		return nil
	}

	if strength < 0 {
		strength = 0
	}
	if strength > 1 {
		strength = 1
	}

	bounds := base.Bounds()
	dst := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			src := base.NRGBAAt(x, y)
			maskAlpha := float64(edgeMask.GrayAt(x, y).Y) / 255.0

			overlayAlpha := maskAlpha * strength * (float64(darken.A) / 255.0)
			if overlayAlpha < 0 {
				overlayAlpha = 0
			}
			if overlayAlpha > 1 {
				overlayAlpha = 1
			}

			blend := func(srcVal, overlayVal uint8) uint8 {
				return uint8(math.Round((1-overlayAlpha)*float64(srcVal) + overlayAlpha*float64(overlayVal)))
			}

			dst.SetNRGBA(x, y, color.NRGBA{
				R: blend(src.R, darken.R),
				G: blend(src.G, darken.G),
				B: blend(src.B, darken.B),
				A: src.A, // preserve base alpha
			})
		}
	}

	return dst
}
