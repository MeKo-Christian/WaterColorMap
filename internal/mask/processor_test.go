package mask

import (
	"image"
	"image/color"
	"testing"
)

// TestExtractBinaryMask tests extracting a binary mask from a colored layer image
func TestExtractBinaryMask(t *testing.T) {
	// Create a test image with a specific color feature
	// Background should be transparent, feature should be a specific color
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Fill background with transparent pixels
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 0})
		}
	}

	// Draw a blue square in the center (like a water feature)
	// This represents a rendered layer with a specific color
	for y := 3; y < 7; y++ {
		for x := 3; x < 7; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255}) // Blue
		}
	}

	// Extract binary mask - should be white where feature is, black elsewhere
	mask := ExtractBinaryMask(img)

	// Verify mask dimensions match input
	if mask.Bounds() != img.Bounds() {
		t.Errorf("mask bounds %v != image bounds %v", mask.Bounds(), img.Bounds())
	}

	// Verify background pixels are black (no feature)
	bgPixel := mask.GrayAt(0, 0)
	if bgPixel.Y != 0 {
		t.Errorf("background pixel should be black (0), got %d", bgPixel.Y)
	}

	// Verify feature pixels are white (255)
	featurePixel := mask.GrayAt(5, 5)
	if featurePixel.Y != 255 {
		t.Errorf("feature pixel should be white (255), got %d", featurePixel.Y)
	}

	// Verify edge transition
	edgePixel := mask.GrayAt(2, 5) // Just outside feature
	if edgePixel.Y != 0 {
		t.Errorf("edge pixel outside feature should be black (0), got %d", edgePixel.Y)
	}
}

// TestGaussianBlur tests applying Gaussian blur to soften mask edges
func TestGaussianBlur(t *testing.T) {
	// Create a simple binary mask with a sharp edge
	mask := image.NewGray(image.Rect(0, 0, 10, 10))

	// Left half black (0), right half white (255)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if x < 5 {
				mask.SetGray(x, y, color.Gray{Y: 0})
			} else {
				mask.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	// Apply Gaussian blur with radius 1.0
	blurred := GaussianBlur(mask, 1.0)

	// Verify dimensions preserved
	if blurred.Bounds() != mask.Bounds() {
		t.Errorf("blurred bounds %v != mask bounds %v", blurred.Bounds(), mask.Bounds())
	}

	// Verify far left is still dark (should remain mostly black)
	leftPixel := blurred.GrayAt(0, 5)
	if leftPixel.Y > 50 {
		t.Errorf("far left pixel should be dark (<50), got %d", leftPixel.Y)
	}

	// Verify far right is still bright (should remain mostly white)
	rightPixel := blurred.GrayAt(9, 5)
	if rightPixel.Y < 200 {
		t.Errorf("far right pixel should be bright (>200), got %d", rightPixel.Y)
	}

	// Verify edge is now gradual (middle pixels should be gray)
	edgePixel := blurred.GrayAt(5, 5)
	if edgePixel.Y < 50 || edgePixel.Y > 200 {
		t.Errorf("edge pixel should be gray (50-200), got %d", edgePixel.Y)
	}
}

// TestGeneratePerlinNoise tests generating tileable Perlin noise
func TestGeneratePerlinNoise(t *testing.T) {
	width := 256
	height := 256
	scale := 50.0

	// Generate Perlin noise texture
	noise := GeneratePerlinNoise(width, height, scale, 42)

	// Verify dimensions
	bounds := noise.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Errorf("noise dimensions %dx%d != expected %dx%d", bounds.Dx(), bounds.Dy(), width, height)
	}

	// Verify it's not all the same value (has variation)
	firstPixel := noise.GrayAt(0, 0).Y
	foundDifferent := false
	for y := 0; y < height && !foundDifferent; y++ {
		for x := 0; x < width && !foundDifferent; x++ {
			if noise.GrayAt(x, y).Y != firstPixel {
				foundDifferent = true
			}
		}
	}
	if !foundDifferent {
		t.Error("noise should have variation, but all pixels are the same")
	}

	// Verify values are in valid range (0-255)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := noise.GrayAt(x, y).Y
			if val < 0 || val > 255 {
				t.Errorf("noise value %d at (%d,%d) out of range 0-255", val, x, y)
			}
		}
	}

	// Verify determinism - same seed produces same noise
	noise2 := GeneratePerlinNoise(width, height, scale, 42)
	pixel1 := noise.GrayAt(100, 100).Y
	pixel2 := noise2.GrayAt(100, 100).Y
	if pixel1 != pixel2 {
		t.Errorf("same seed should produce same noise: %d != %d", pixel1, pixel2)
	}

	// Verify different seeds produce different noise (check multiple pixels)
	noise3 := GeneratePerlinNoise(width, height, scale, 99)
	differentCount := 0
	sampleCount := 0
	for y := 0; y < height; y += 10 {
		for x := 0; x < width; x += 10 {
			sampleCount++
			if noise.GrayAt(x, y).Y != noise3.GrayAt(x, y).Y {
				differentCount++
			}
		}
	}
	// At least 80% of sampled pixels should be different
	if float64(differentCount)/float64(sampleCount) < 0.8 {
		t.Errorf("different seeds should produce mostly different noise, only %d/%d pixels different", differentCount, sampleCount)
	}
}

// TestApplyNoiseToMask tests overlaying noise on a blurred mask
func TestApplyNoiseToMask(t *testing.T) {
	// Create a simple gradient mask (simulating a blurred edge)
	mask := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			// Gradient from left (black) to right (white)
			gray := uint8(float64(x) / 100.0 * 255.0)
			mask.SetGray(x, y, color.Gray{Y: gray})
		}
	}

	// Generate simple noise
	noise := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			// Simple checkerboard pattern as "noise"
			if (x+y)%2 == 0 {
				noise.SetGray(x, y, color.Gray{Y: 200})
			} else {
				noise.SetGray(x, y, color.Gray{Y: 100})
			}
		}
	}

	// Apply noise with 50% strength
	result := ApplyNoiseToMask(mask, noise, 0.5)

	// Verify dimensions
	if result.Bounds() != mask.Bounds() {
		t.Errorf("result bounds %v != mask bounds %v", result.Bounds(), mask.Bounds())
	}

	// Verify left side (black mask) stays mostly dark even with noise
	leftPixel := result.GrayAt(10, 50)
	if leftPixel.Y > 100 {
		t.Errorf("left pixel should stay dark (<100), got %d", leftPixel.Y)
	}

	// Verify right side (white mask) has some variation from noise
	rightPixel1 := result.GrayAt(95, 50)
	rightPixel2 := result.GrayAt(96, 50)
	if rightPixel1.Y == rightPixel2.Y {
		t.Error("noise should create variation in bright areas")
	}

	// Verify result is not identical to input (noise was applied)
	middleOriginal := mask.GrayAt(50, 50).Y
	middleResult := result.GrayAt(50, 50).Y
	if middleOriginal == middleResult {
		t.Error("noise should modify the mask values")
	}
}

// TestApplyThreshold tests thresholding to sharpen mask edges
func TestApplyThreshold(t *testing.T) {
	// Create a gradient mask (soft edge)
	mask := image.NewGray(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			// Gradient: 0, 25, 51, 76, 102, 127, 153, 178, 204, 229
			gray := uint8(float64(x) * 25.5)
			mask.SetGray(x, y, color.Gray{Y: gray})
		}
	}

	// Apply threshold at 128
	result := ApplyThreshold(mask, 128)

	// Verify dimensions
	if result.Bounds() != mask.Bounds() {
		t.Errorf("result bounds %v != mask bounds %v", result.Bounds(), mask.Bounds())
	}

	// Verify values below threshold become black
	for x := 0; x < 5; x++ {
		pixel := result.GrayAt(x, 5)
		if pixel.Y != 0 {
			t.Errorf("pixel at x=%d should be 0 (below threshold), got %d", x, pixel.Y)
		}
	}

	// Verify values above threshold become white
	for x := 6; x < 10; x++ {
		pixel := result.GrayAt(x, 5)
		if pixel.Y != 255 {
			t.Errorf("pixel at x=%d should be 255 (above threshold), got %d", x, pixel.Y)
		}
	}
}

// TestAntialiasEdges tests applying subtle antialiasing to mask edges
func TestAntialiasEdges(t *testing.T) {
	// Create a binary mask with sharp edges
	mask := image.NewGray(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if x < 5 {
				mask.SetGray(x, y, color.Gray{Y: 0})
			} else {
				mask.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	// Apply antialiasing
	result := AntialiasEdges(mask, 0.5)

	// Verify dimensions
	if result.Bounds() != mask.Bounds() {
		t.Errorf("result bounds %v != mask bounds %v", result.Bounds(), mask.Bounds())
	}

	// Verify far left is still black
	if result.GrayAt(0, 5).Y > 50 {
		t.Errorf("far left should stay dark, got %d", result.GrayAt(0, 5).Y)
	}

	// Verify far right is still white
	if result.GrayAt(9, 5).Y < 200 {
		t.Errorf("far right should stay bright, got %d", result.GrayAt(9, 5).Y)
	}

	// Verify edge pixels are smoothed (not pure black or white)
	edgePixel := result.GrayAt(5, 5)
	if edgePixel.Y == 0 || edgePixel.Y == 255 {
		t.Errorf("edge pixel should be antialiased (gray), got %d", edgePixel.Y)
	}
}

// TestWatercolorPipeline tests the complete watercolor effect pipeline
func TestWatercolorPipeline(t *testing.T) {
	// Create a test layer image with a blue feature
	layerImg := image.NewRGBA(image.Rect(0, 0, 256, 256))

	// Fill background with transparent
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			layerImg.Set(x, y, color.RGBA{0, 0, 0, 0})
		}
	}

	// Draw a blue circle in the center (simulate a water body)
	centerX, centerY := 128, 128
	radius := 50
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			dx := x - centerX
			dy := y - centerY
			if dx*dx+dy*dy <= radius*radius {
				layerImg.Set(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}

	// Step 1: Extract binary mask
	mask := ExtractBinaryMask(layerImg)
	if mask == nil {
		t.Fatal("ExtractBinaryMask returned nil")
	}

	// Step 2: Apply Gaussian blur
	blurred := GaussianBlur(mask, 2.0)
	if blurred == nil {
		t.Fatal("GaussianBlur returned nil")
	}

	// Step 3: Generate Perlin noise
	noise := GeneratePerlinNoise(256, 256, 30.0, 12345)
	if noise == nil {
		t.Fatal("GeneratePerlinNoise returned nil")
	}

	// Step 4: Apply noise to blurred mask
	noisy := ApplyNoiseToMask(blurred, noise, 0.3)
	if noisy == nil {
		t.Fatal("ApplyNoiseToMask returned nil")
	}

	// Step 5: Apply threshold
	thresholded := ApplyThreshold(noisy, 128)
	if thresholded == nil {
		t.Fatal("ApplyThreshold returned nil")
	}

	// Step 6: Apply antialiasing
	final := AntialiasEdges(thresholded, 0.5)
	if final == nil {
		t.Fatal("AntialiasEdges returned nil")
	}

	// Verify final result
	if final.Bounds().Dx() != 256 || final.Bounds().Dy() != 256 {
		t.Errorf("final dimensions incorrect: got %dx%d, want 256x256",
			final.Bounds().Dx(), final.Bounds().Dy())
	}

	// Verify background is dark (no feature)
	bgPixel := final.GrayAt(10, 10)
	if bgPixel.Y > 50 {
		t.Errorf("background should be dark (<50), got %d", bgPixel.Y)
	}

	// Verify feature center is bright
	centerPixel := final.GrayAt(128, 128)
	if centerPixel.Y < 200 {
		t.Errorf("feature center should be bright (>200), got %d", centerPixel.Y)
	}

	// Verify there's a gradient at the edge (watercolor effect)
	// Check a few pixels at different distances from edge
	foundGradient := false
	for offset := 0; offset < 15; offset++ {
		pixelVal := final.GrayAt(128+radius+offset, 128).Y
		// If we find a pixel that's not pure black or white, we have gradient
		if pixelVal > 20 && pixelVal < 235 {
			foundGradient = true
			break
		}
	}
	if !foundGradient {
		t.Error("should have gradient at feature edge (watercolor effect)")
	}
}
