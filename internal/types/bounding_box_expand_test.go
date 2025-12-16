package types

import "testing"

func TestBoundingBoxExpandByFraction(t *testing.T) {
	b := BoundingBox{MinLon: 10, MinLat: 20, MaxLon: 30, MaxLat: 40}

	expanded := b.ExpandByFraction(0.1)
	// width=20, height=20 => delta=2 on each side
	if expanded.MinLon != 8 || expanded.MaxLon != 32 || expanded.MinLat != 18 || expanded.MaxLat != 42 {
		t.Fatalf("unexpected expanded bbox: %+v", expanded)
	}

	unchanged := b.ExpandByFraction(0)
	if unchanged != b {
		t.Fatalf("expected unchanged bbox, got %+v", unchanged)
	}
}
