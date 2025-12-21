package texture

import (
	"image"
	"image/color"
	"math"

	"github.com/MeKo-Tech/watercolormap/internal/geojson"
)

// DefaultLayerTextures maps layer types to their default texture filenames.
var DefaultLayerTextures = map[geojson.LayerType]string{
	geojson.LayerLand:     "land.png",
	geojson.LayerWater:    "water.png",
	geojson.LayerParks:    "green.png",
	geojson.LayerCivic:    "lilac.png",
	geojson.LayerRoads:    "gray.png",
	geojson.LayerHighways: "yellow.png",
	geojson.LayerPaper:    "white.png",
}

// TextureNameForLayer returns the default texture filename for a layer.
func TextureNameForLayer(layer geojson.LayerType) (string, bool) {
	name, ok := DefaultLayerTextures[layer]
	return name, ok
}

// TileTexture tiles a source texture into a square tile of the given size.
// Offsets align the sampling to a global texture grid to keep seams invisible across tiles.
func TileTexture(src image.Image, tileSize int, offsetX, offsetY int) *image.NRGBA {
	if src == nil || tileSize <= 0 {
		return nil
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width == 0 || height == 0 {
		return image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	}

	dst := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))

	mod := func(a, b int) int {
		r := a % b
		if r < 0 {
			r += b
		}
		return r
	}

	for y := 0; y < tileSize; y++ {
		sy := bounds.Min.Y + mod(offsetY+y, height)
		for x := 0; x < tileSize; x++ {
			sx := bounds.Min.X + mod(offsetX+x, width)
			dst.Set(x, y, src.At(sx, sy))
		}
	}

	return dst
}

// ApplyMaskToTexture applies a grayscale mask as the alpha channel to a texture.
// The texture is tiled if smaller than the mask to avoid seams at the edges.
func ApplyMaskToTexture(tex image.Image, mask *image.Gray) *image.NRGBA {
	if tex == nil || mask == nil {
		return nil
	}

	dst := image.NewNRGBA(mask.Bounds())

	texBounds := tex.Bounds()
	texW := texBounds.Dx()
	texH := texBounds.Dy()

	if texW == 0 || texH == 0 {
		return dst
	}

	mod := func(a, b int) int {
		r := a % b
		if r < 0 {
			r += b
		}
		return r
	}

	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		sy := texBounds.Min.Y + mod(y, texH)
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			sx := texBounds.Min.X + mod(x, texW)

			c := color.NRGBAModel.Convert(tex.At(sx, sy)).(color.NRGBA)
			// Mask controls the alpha channel; RGB comes from the texture
			c.A = mask.GrayAt(x, y).Y
			dst.SetNRGBA(x, y, c)
		}
	}

	return dst
}

// TintTexture overlays a tint color onto a texture with the given strength (0-1).
// The alpha channel is preserved from the original texture.
func TintTexture(tex image.Image, tint color.NRGBA, strength float64) *image.NRGBA {
	if tex == nil {
		return nil
	}

	if strength < 0 {
		strength = 0
	}
	if strength > 1 {
		strength = 1
	}

	bounds := tex.Bounds()
	dst := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			srcColor := color.NRGBAModel.Convert(tex.At(x, y)).(color.NRGBA)

			blend := func(src, tgt uint8) uint8 {
				val := (1.0-strength)*float64(src) + strength*float64(tgt)
				return uint8(math.Round(val))
			}

			dst.SetNRGBA(x, y, color.NRGBA{
				R: blend(srcColor.R, tint.R),
				G: blend(srcColor.G, tint.G),
				B: blend(srcColor.B, tint.B),
				A: srcColor.A, // Preserve original alpha; tint applies to color only
			})
		}
	}

	return dst
}
