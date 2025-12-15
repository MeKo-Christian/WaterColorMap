package mask

import (
	"image"
	"image/color"
	"math"

	"github.com/aquilax/go-perlin"
	"github.com/disintegration/gift"
)

// ExtractBinaryMask converts a colored layer image into a binary mask.
// Pixels with any non-transparent color become white (255), transparent pixels become black (0).
func ExtractBinaryMask(img image.Image) *image.Gray {
	bounds := img.Bounds()
	mask := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			// RGBA() returns values in range 0-65535, so check if alpha > 0
			if a > 0 {
				mask.SetGray(x, y, color.Gray{Y: 255}) // White - feature present
			} else {
				mask.SetGray(x, y, color.Gray{Y: 0}) // Black - no feature
			}
		}
	}

	return mask
}

// GaussianBlur applies a Gaussian blur filter to soften mask edges.
// The sigma parameter controls the blur radius (larger = more blur).
func GaussianBlur(mask *image.Gray, sigma float32) *image.Gray {
	g := gift.New(gift.GaussianBlur(sigma))

	// Create output image
	dst := image.NewGray(g.Bounds(mask.Bounds()))

	// Apply the blur filter
	g.Draw(dst, mask)

	return dst
}

// GeneratePerlinNoise generates a grayscale Perlin noise texture.
// width, height: dimensions of the output image
// scale: controls the frequency of the noise (smaller = more detail)
// seed: random seed for deterministic noise generation
func GeneratePerlinNoise(width, height int, scale float64, seed int64) *image.Gray {
	// Create Perlin noise generator with octaves, alpha, and beta parameters
	// alpha: persistence (how much each octave contributes)
	// beta: lacunarity (frequency multiplier between octaves)
	// n: number of octaves
	p := perlin.NewPerlin(2.0, 2.0, 3, seed)

	noise := image.NewGray(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Sample Perlin noise at normalized coordinates
			nx := float64(x) / scale
			ny := float64(y) / scale

			// Get noise value (range approximately -1 to 1)
			val := p.Noise2D(nx, ny)

			// Normalize to 0-255 range
			// Apply a gentler mapping to get better distribution
			normalized := (val + 1.0) / 2.0
			gray := uint8(math.Max(0, math.Min(255, normalized*255)))

			noise.SetGray(x, y, color.Gray{Y: gray})
		}
	}

	return noise
}

// ApplyNoiseToMask overlays Perlin noise onto a blurred mask to create organic edges.
// mask: the blurred binary mask
// noise: the Perlin noise texture (should match or be larger than mask dimensions)
// strength: how much noise to apply (0.0 = no noise, 1.0 = full noise)
func ApplyNoiseToMask(mask, noise *image.Gray, strength float64) *image.Gray {
	bounds := mask.Bounds()
	result := image.NewGray(bounds)

	noiseBounds := noise.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Get mask value
			maskVal := float64(mask.GrayAt(x, y).Y)

			// Get noise value (tile if noise is smaller, or sample if larger)
			nx := (x - bounds.Min.X) % noiseBounds.Dx()
			ny := (y - bounds.Min.Y) % noiseBounds.Dy()
			noiseVal := float64(noise.GrayAt(noiseBounds.Min.X+nx, noiseBounds.Min.Y+ny).Y)

			// Apply noise as a perturbation
			// Noise is centered around 128, so subtract 128 to get -128 to +127 range
			noiseDelta := (noiseVal - 128.0) * strength

			// Combine mask and noise
			combined := maskVal + noiseDelta

			// Clamp to valid range
			if combined < 0 {
				combined = 0
			}
			if combined > 255 {
				combined = 255
			}

			result.SetGray(x, y, color.Gray{Y: uint8(combined)})
		}
	}

	return result
}

// ApplyThreshold applies a binary threshold to sharpen mask edges.
// Values below threshold become 0 (black), values at or above become 255 (white).
func ApplyThreshold(mask *image.Gray, threshold uint8) *image.Gray {
	bounds := mask.Bounds()
	result := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			val := mask.GrayAt(x, y).Y

			if val >= threshold {
				result.SetGray(x, y, color.Gray{Y: 255})
			} else {
				result.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return result
}

// AntialiasEdges applies subtle antialiasing to smooth sharp mask edges.
// This is essentially a light Gaussian blur to soften transitions.
func AntialiasEdges(mask *image.Gray, sigma float32) *image.Gray {
	// Reuse GaussianBlur with a small sigma for antialiasing
	return GaussianBlur(mask, sigma)
}
