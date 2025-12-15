package mask

import (
	"image"
	"image/color"
	"testing"
)

func TestCreateEdgeMaskProducesHalo(t *testing.T) {
	// Create a circular mask
	size := 64
	mask := image.NewGray(image.Rect(0, 0, size, size))
	cx, cy := 32, 32
	radius := 16
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= radius*radius {
				mask.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	edge := CreateEdgeMask(mask, 1.0, 3.0)
	if edge == nil {
		t.Fatal("CreateEdgeMask returned nil")
	}

	// Center should be near zero; edge region should have signal; outside should be near zero.
	center := edge.GrayAt(cx, cy).Y
	if center > 10 {
		t.Fatalf("expected center halo near zero, got %d", center)
	}

	outside := edge.GrayAt(0, 0).Y
	if outside > 10 {
		t.Fatalf("expected outside halo near zero, got %d", outside)
	}

	edgeSignalFound := false
	for y := 0; y < size && !edgeSignalFound; y++ {
		for x := 0; x < size && !edgeSignalFound; x++ {
			if edge.GrayAt(x, y).Y > 20 {
				edgeSignalFound = true
			}
		}
	}
	if !edgeSignalFound {
		t.Fatalf("expected non-zero halo values near the feature edge")
	}
}

func TestTaperEdgeMask(t *testing.T) {
	mask := image.NewGray(image.Rect(0, 0, 2, 1))
	mask.SetGray(0, 0, color.Gray{Y: 128})
	mask.SetGray(1, 0, color.Gray{Y: 255})

	tapered := TaperEdgeMask(mask, 2.0)
	if tapered == nil {
		t.Fatal("TaperEdgeMask returned nil")
	}

	// 128/255 squared ~0.252 -> ~64
	if got := tapered.GrayAt(0, 0).Y; got < 50 || got > 80 {
		t.Fatalf("unexpected tapered value for mid-gray: %d", got)
	}
	// 255 should stay at 255
	if got := tapered.GrayAt(1, 0).Y; got != 255 {
		t.Fatalf("unexpected tapered value for white: %d", got)
	}
}

func TestApplyEdgeDarkening(t *testing.T) {
	base := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			base.SetNRGBA(x, y, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}

	edgeMask := image.NewGray(image.Rect(0, 0, 3, 3))
	edgeMask.SetGray(1, 1, color.Gray{Y: 0})   // center
	edgeMask.SetGray(0, 0, color.Gray{Y: 255}) // corner (edge)

	darken := color.NRGBA{R: 50, G: 40, B: 30, A: 255}
	result := ApplyEdgeDarkening(base, edgeMask, darken, 0.5)
	if result == nil {
		t.Fatal("ApplyEdgeDarkening returned nil")
	}

	center := result.NRGBAAt(1, 1)
	if center.R != 200 || center.G != 200 || center.B != 200 {
		t.Fatalf("center should remain unchanged, got %+v", center)
	}

	darkened := result.NRGBAAt(0, 0)
	if !(darkened.R < 200 && darkened.G < 200 && darkened.B < 200) {
		t.Fatalf("edge pixel should be darkened, got %+v", darkened)
	}
	if darkened.A != 255 {
		t.Fatalf("alpha should be preserved, got %d", darkened.A)
	}
}
