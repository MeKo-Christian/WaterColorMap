package mask

import (
	"image"
	"image/color"
	"math"
)

// DistanceContext holds reusable buffers for distance transform operations.
// Reusing these buffers across multiple calls significantly reduces allocations.
type DistanceContext struct {
	// Buffers for distanceTransform1D
	v []int     // parabola vertex positions
	z []float64 // intersection x-coordinates

	// Buffers for EuclideanDistanceTransform
	temp   []float64 // squared distances (flat 1D: y*width+x)
	isEdge []bool    // edge detection (flat 1D: y*width+x)
	rowBuf []float64 // row input/output buffer
	colBuf []float64 // column input/output buffer
}

// NewDistanceContext creates a context sized for images up to maxDim x maxDim.
func NewDistanceContext(maxDim int) *DistanceContext {
	return &DistanceContext{
		v:      make([]int, maxDim),
		z:      make([]float64, maxDim+1),
		temp:   make([]float64, maxDim*maxDim),
		isEdge: make([]bool, maxDim*maxDim),
		rowBuf: make([]float64, maxDim),
		colBuf: make([]float64, maxDim),
	}
}

// EnsureCapacity grows buffers if needed for the given dimensions.
func (c *DistanceContext) EnsureCapacity(width, height int) {
	maxDim := width
	if height > maxDim {
		maxDim = height
	}
	area := width * height

	if len(c.v) < maxDim {
		c.v = make([]int, maxDim)
	}
	if len(c.z) < maxDim+1 {
		c.z = make([]float64, maxDim+1)
	}
	if len(c.temp) < area {
		c.temp = make([]float64, area)
	}
	if len(c.isEdge) < area {
		c.isEdge = make([]bool, area)
	}
	if len(c.rowBuf) < width {
		c.rowBuf = make([]float64, width)
	}
	if len(c.colBuf) < height {
		c.colBuf = make([]float64, height)
	}
}

// EuclideanDistanceTransform computes the Euclidean distance from each "inside"
// pixel (value > 0) to the nearest boundary (value == 0) using the Felzenszwalb
// & Huttenlocher separable squared distance transform algorithm.
//
// Returns distances normalized to 0-255 range, where:
//   - 0 = at boundary (edge)
//   - 255 = maximum distance (center of feature)
//
// The maxDistance parameter caps the distance calculation for normalization.
// For example, maxDistance=50.0 means distances are normalized such that
// 50 pixels from the edge maps to 255.
//
// Algorithm: O(n) complexity using two separable 1D passes (horizontal, vertical)
// with parabola lower envelope method.
func EuclideanDistanceTransform(mask *image.Gray, maxDistance float64) *image.Gray {
	bounds := mask.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create temporary context for this call
	ctx := NewDistanceContext(max(width, height))
	return EuclideanDistanceTransformWithContext(mask, maxDistance, ctx)
}

// EuclideanDistanceTransformWithContext is like EuclideanDistanceTransform but uses
// preallocated buffers from the provided context to avoid allocations.
func EuclideanDistanceTransformWithContext(mask *image.Gray, maxDistance float64, ctx *DistanceContext) *image.Gray {
	bounds := mask.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Ensure context has enough capacity
	ctx.EnsureCapacity(width, height)

	infinity := maxDistance * maxDistance * 2.0

	// Use flat 1D slices from context
	temp := ctx.temp
	isEdge := ctx.isEdge

	// Clear the isEdge buffer (temp will be overwritten completely)
	for i := 0; i < width*height; i++ {
		isEdge[i] = false
	}

	// First, detect which inside pixels are at the edge (adjacent to background)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := mask.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y
			if val > 0 {
				// Check if any 4-connected neighbor is background (value == 0)
				isEdgePixel := false
				// Check left
				if x > 0 && mask.GrayAt(bounds.Min.X+x-1, bounds.Min.Y+y).Y == 0 {
					isEdgePixel = true
				}
				// Check right
				if x < width-1 && mask.GrayAt(bounds.Min.X+x+1, bounds.Min.Y+y).Y == 0 {
					isEdgePixel = true
				}
				// Check top
				if y > 0 && mask.GrayAt(bounds.Min.X+x, bounds.Min.Y+y-1).Y == 0 {
					isEdgePixel = true
				}
				// Check bottom
				if y < height-1 && mask.GrayAt(bounds.Min.X+x, bounds.Min.Y+y+1).Y == 0 {
					isEdgePixel = true
				}
				isEdge[y*width+x] = isEdgePixel
			}
		}
	}

	// Now initialize based on edge detection
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			val := mask.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y
			if val > 0 {
				if isEdge[idx] {
					temp[idx] = 0.0 // Edge pixel - distance is 0
				} else {
					temp[idx] = infinity // Interior pixel - needs distance computed
				}
			} else {
				temp[idx] = infinity // Background pixel - outside the shape
			}
		}
	}

	// Use row/col buffers from context
	rowBuf := ctx.rowBuf
	colBuf := ctx.colBuf

	// First pass: rows (horizontal distances)
	for y := 0; y < height; y++ {
		rowStart := y * width
		// Copy row to buffer
		for x := 0; x < width; x++ {
			rowBuf[x] = temp[rowStart+x]
		}
		// Transform in place using v and z buffers
		distanceTransform1DWithBuffers(rowBuf[:width], rowBuf[:width], ctx.v, ctx.z)
		// Copy back
		for x := 0; x < width; x++ {
			temp[rowStart+x] = rowBuf[x]
		}
	}

	// Second pass: columns (complete Euclidean distance)
	for x := 0; x < width; x++ {
		// Extract column to buffer
		for y := 0; y < height; y++ {
			colBuf[y] = temp[y*width+x]
		}
		// Transform in place
		distanceTransform1DWithBuffers(colBuf[:height], colBuf[:height], ctx.v, ctx.z)
		// Write back
		for y := 0; y < height; y++ {
			temp[y*width+x] = colBuf[y]
		}
	}

	// Convert squared distances to distances and normalize to 0-255
	output := image.NewGray(bounds)
	maxDistSq := maxDistance * maxDistance

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			distSq := temp[idx]
			val := mask.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y

			// Background pixels (outside shape) remain at 0
			if val == 0 {
				output.SetGray(bounds.Min.X+x, bounds.Min.Y+y, color.Gray{Y: 0})
				continue
			}

			// Interior pixels: if still at infinity, clamp to maxDistance
			// (this happens when distance exceeds maxDistance)
			if distSq >= infinity/2 {
				output.SetGray(bounds.Min.X+x, bounds.Min.Y+y, color.Gray{Y: 255})
				continue
			}

			// Clamp to maxDistance and normalize
			if distSq >= maxDistSq {
				output.SetGray(bounds.Min.X+x, bounds.Min.Y+y, color.Gray{Y: 255})
			} else {
				dist := math.Sqrt(distSq)
				normalized := uint8(255.0 * dist / maxDistance)
				output.SetGray(bounds.Min.X+x, bounds.Min.Y+y, color.Gray{Y: normalized})
			}
		}
	}

	return output
}

// distanceTransform1D computes the squared distance transform along one dimension
// using the parabola lower envelope method from Felzenszwalb & Huttenlocher.
//
// Input: array of values (0 for inside pixels, infinity for boundary)
// Output: array of squared distances to nearest boundary
func distanceTransform1D(input []float64, output []float64) {
	n := len(input)
	v := make([]int, n)
	z := make([]float64, n+1)
	distanceTransform1DWithBuffers(input, output, v, z)
}

// distanceTransform1DWithBuffers computes the squared distance transform using provided buffers.
// v must have length >= n, z must have length >= n+1 where n = len(input).
// This avoids allocations when called repeatedly.
func distanceTransform1DWithBuffers(input []float64, output []float64, v []int, z []float64) {
	n := len(input)

	k := 0 // Index of rightmost parabola in lower envelope
	v[0] = 0
	z[0] = math.Inf(-1)
	z[1] = math.Inf(1)

	// Build lower envelope of parabolas
	for q := 1; q < n; q++ {
		// Compute intersection of parabola from q with rightmost parabola in envelope
		// Parabola from position i: f_i(x) = (x - i)^2 + input[i]
		// Find intersection s where f_v[k](s) = f_q(s)
		var s float64
		for k >= 0 {
			// Solve (s - v[k])^2 + input[v[k]] = (s - q)^2 + input[q]
			// Expands to: s = ((input[q] + q^2) - (input[v[k]] + v[k]^2)) / (2*(q - v[k]))
			s = ((input[q] + float64(q*q)) - (input[v[k]] + float64(v[k]*v[k]))) /
				(2.0 * float64(q-v[k]))

			if s <= z[k] {
				// Remove this parabola from envelope (it's completely dominated)
				k--
			} else {
				// This parabola stays in envelope
				break
			}
		}

		// Add parabola q to envelope
		k++
		v[k] = q
		z[k] = s
		z[k+1] = math.Inf(1)
	}

	// Sample the lower envelope to get output distances
	k = 0
	for q := 0; q < n; q++ {
		// Find which parabola is minimal at position q
		for z[k+1] < float64(q) {
			k++
		}
		// Compute squared distance: (q - v[k])^2 + input[v[k]]
		dx := float64(q - v[k])
		output[q] = dx*dx + input[v[k]]
	}
}

// DistanceToIntensity converts a distance mask to an intensity mask using
// a power curve falloff: I = pow(1 - D/R, gamma)
//
// Input: distMask with values 0-255 where 0=boundary, 255=max distance
// Output: intensity mask with values 0-255 where 0=max effect (edge), 255=no effect (center)
//
// The gamma parameter controls curve shape:
//   - gamma > 1: steeper falloff near edges (more concentrated darkening)
//   - gamma = 1: linear falloff
//   - gamma < 1: flatter falloff near edges (more diffuse darkening)
//
// The output is suitable for use with ApplySoftEdgeMask or similar edge darkening functions.
func DistanceToIntensity(distMask *image.Gray, gamma float64) *image.Gray {
	bounds := distMask.Bounds()
	output := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Get normalized distance (0-255)
			distNorm := float64(distMask.GrayAt(x, y).Y) / 255.0

			// I = pow(1 - D/R, gamma)
			base := math.Max(0, 1.0-distNorm)
			intensity := math.Pow(base, gamma)

			// Convert intensity (0-1) to output (0-255)
			// Invert: 0 intensity = 255 output (no darkening at center)
			//         1 intensity = 0 output (max darkening at edge)
			outputVal := uint8(255.0 * (1.0 - intensity))
			output.SetGray(x, y, color.Gray{Y: outputVal})
		}
	}

	return output
}

// CreateDistanceEdgeMask is a high-level convenience function that combines
// distance transform and intensity mapping in a single call.
//
// It computes the Euclidean distance transform and applies a power curve falloff
// to create an edge mask suitable for edge darkening effects.
//
// Parameters:
//   - mask: Binary mask (0=outside/boundary, >0=inside)
//   - radius: Distance parameter in pixels (controls how far the effect extends)
//   - gamma: Power curve exponent (>1 for steeper falloff, <1 for gentler falloff)
//
// Returns: Grayscale mask where 0=max darkening (at edges), 255=no darkening (at center)
func CreateDistanceEdgeMask(mask *image.Gray, radius float64, gamma float64) *image.Gray {
	distMask := EuclideanDistanceTransform(mask, radius)
	return DistanceToIntensity(distMask, gamma)
}

// CreateDistanceEdgeMaskWithContext is like CreateDistanceEdgeMask but uses
// preallocated buffers from the provided context to avoid allocations.
func CreateDistanceEdgeMaskWithContext(mask *image.Gray, radius float64, gamma float64, ctx *DistanceContext) *image.Gray {
	distMask := EuclideanDistanceTransformWithContext(mask, radius, ctx)
	return DistanceToIntensity(distMask, gamma)
}
