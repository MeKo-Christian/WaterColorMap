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

	inner := BoxBlurSigma(mask, innerSigma)
	outer := BoxBlurSigma(mask, outerSigma)

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

	// Invert mask in place: edges should be bright (255), centers dark (0).
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			val := dst.GrayAt(x, y).Y
			dst.SetGray(x, y, color.Gray{Y: 255 - val})
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

// ApplySoftEdgeMask applies a soft edge darkening effect while increasing saturation at edges.
// The mask controls effect intensity: 255 (white) = no change, 0 (black) = maximum darkening + saturation.
// This simulates watercolor pigment concentrating at edges - both darker and more vibrant.
// The strength parameter (0.0-1.0) controls how aggressive the effect is.
func ApplySoftEdgeMask(base *image.NRGBA, mask *image.Gray, strength float64) *image.NRGBA {
	if base == nil || mask == nil {
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
			maskVal := int(mask.GrayAt(x, y).Y)

			// Quadratic falloff: creates softer, more natural transition
			// Effect amount: 0 at white (center), strength at black (edges)
			// maskVal^2 / 255^2 gives normalized squared value
			maskSquared := maskVal * maskVal                     // max: 65025
			invMaskSquared := 65025 - maskSquared                // 255*255 = 65025
			effectInt := int(float64(invMaskSquared) * strength) // 0..65025

			// Convert RGB to HSL (integer-only)
			h, s, l := rgbToHSL(src.R, src.G, src.B)

			// Darken by reducing lightness
			// l_new = l * (1 - effect) = l * (65025 - effectInt) / 65025
			darkening := 65025 - effectInt
			lNew := uint8((int(l) * darkening) / 65025)

			// Convert back to RGB (integer-only)
			r, g, b := hslToRGB(h, s, lNew)

			dst.SetNRGBA(x, y, color.NRGBA{
				R: r,
				G: g,
				B: b,
				A: src.A, // preserve original alpha
			})
		}
	}

	return dst
}

// MultiplyRGBByMask multiplies the RGB color values of an image by a grayscale mask.
// The mask values (0-255) are normalized to (0-1) and multiplied with RGB values.
// Alpha channel is preserved from the base image.
func MultiplyRGBByMask(base *image.NRGBA, mask *image.Gray) *image.NRGBA {
	if base == nil || mask == nil {
		return nil
	}

	bounds := base.Bounds()
	dst := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			src := base.NRGBAAt(x, y)
			maskVal := float64(mask.GrayAt(x, y).Y) / 255.0

			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(math.Round(float64(src.R) * maskVal)),
				G: uint8(math.Round(float64(src.G) * maskVal)),
				B: uint8(math.Round(float64(src.B) * maskVal)),
				A: src.A, // preserve base alpha
			})
		}
	}

	return dst
}
