package server

import "testing"

func TestParseTilePath(t *testing.T) {
	t.Run("base tile", func(t *testing.T) {
		coords, suffix, ok := parseTilePath("/tiles/z13_x4317_y2692.png")
		if !ok {
			t.Fatalf("expected ok")
		}
		if suffix != "" {
			t.Fatalf("expected empty suffix, got %q", suffix)
		}
		if coords.String() != "z13_x4317_y2692" {
			t.Fatalf("unexpected coords: %s", coords.String())
		}
	})

	t.Run("hidpi tile", func(t *testing.T) {
		coords, suffix, ok := parseTilePath("/tiles/z5_x1_y2@2x.png")
		if !ok {
			t.Fatalf("expected ok")
		}
		if suffix != "@2x" {
			t.Fatalf("expected @2x suffix, got %q", suffix)
		}
		if coords.String() != "z5_x1_y2" {
			t.Fatalf("unexpected coords: %s", coords.String())
		}
	})

	t.Run("reject non-png", func(t *testing.T) {
		_, _, ok := parseTilePath("/tiles/z5_x1_y2.jpg")
		if ok {
			t.Fatalf("expected not ok")
		}
	})

	t.Run("reject other prefix", func(t *testing.T) {
		_, _, ok := parseTilePath("/demo/z5_x1_y2.png")
		if ok {
			t.Fatalf("expected not ok")
		}
	})
}
