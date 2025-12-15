package types

import (
	"math"
	"testing"

	"github.com/paulmach/orb/maptile"
)

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestTileToBounds_ConsistentWithMaptile(t *testing.T) {
	tests := []TileCoordinate{
		{Zoom: 0, X: 0, Y: 0},
		{Zoom: 13, X: 4317, Y: 2692},
		{Zoom: 13, X: 4318, Y: 2692},
		{Zoom: 13, X: 4317, Y: 2693},
		{Zoom: 8, X: 134, Y: 84},
	}

	const eps = 1e-6

	for _, tc := range tests {
		tc := tc
		t.Run(tc.String(), func(t *testing.T) {
			got := TileToBounds(tc)

			mt := maptile.New(uint32(tc.X), uint32(tc.Y), maptile.Zoom(tc.Zoom))
			b := mt.Bound()

			wantMinLon := b.Min.Lon()
			wantMinLat := b.Min.Lat()
			wantMaxLon := b.Max.Lon()
			wantMaxLat := b.Max.Lat()

			if !almostEqual(got.MinLon, wantMinLon, eps) ||
				!almostEqual(got.MinLat, wantMinLat, eps) ||
				!almostEqual(got.MaxLon, wantMaxLon, eps) ||
				!almostEqual(got.MaxLat, wantMaxLat, eps) {
				t.Fatalf(
					"TileToBounds mismatch.\nGot:  minLat=%.9f minLon=%.9f maxLat=%.9f maxLon=%.9f\nWant: minLat=%.9f minLon=%.9f maxLat=%.9f maxLon=%.9f",
					got.MinLat, got.MinLon, got.MaxLat, got.MaxLon,
					wantMinLat, wantMinLon, wantMaxLat, wantMaxLon,
				)
			}
		})
	}
}
