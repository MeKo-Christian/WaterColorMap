package mask

import (
	"image"
	"image/color"
	"math"
)

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
