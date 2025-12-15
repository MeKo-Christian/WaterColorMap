package tile

import (
	"math"
	"testing"
)

func TestCoordsString(t *testing.T) {
	tests := []struct {
		coords   Coords
		expected string
	}{
		{Coords{Z: 13, X: 4297, Y: 2754}, "z13_x4297_y2754"},
		{Coords{Z: 0, X: 0, Y: 0}, "z0_x0_y0"},
		{Coords{Z: 18, X: 12345, Y: 67890}, "z18_x12345_y67890"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.coords.String()
			if result != tt.expected {
				t.Errorf("String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCoordsPath(t *testing.T) {
	coords := Coords{Z: 13, X: 4297, Y: 2754}

	tests := []struct {
		ext      string
		expected string
	}{
		{"png", "z13_x4297_y2754.png"},
		{"json", "z13_x4297_y2754.json"},
		{"xml", "z13_x4297_y2754.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := coords.Path(tt.ext)
			if result != tt.expected {
				t.Errorf("Path(%s) = %s, want %s", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestCoordsBounds(t *testing.T) {
	// Test tile covering Hanover (z13_x4297_y2754)
	coords := Coords{Z: 13, X: 4297, Y: 2754}
	bounds := coords.Bounds()

	t.Logf("Tile %s bounds: [%.6f, %.6f, %.6f, %.6f]",
		coords.String(), bounds[0], bounds[1], bounds[2], bounds[3])

	// Verify bounds are in reasonable range for Germany/Europe
	// Should be somewhere in Central Europe
	if bounds[0] < -10.0 || bounds[0] > 40.0 {
		t.Errorf("minLon %.6f is outside expected range for Europe", bounds[0])
	}
	if bounds[1] < 35.0 || bounds[1] > 70.0 {
		t.Errorf("minLat %.6f is outside expected range for Europe", bounds[1])
	}

	// Verify bounds are ordered correctly
	if bounds[0] >= bounds[2] {
		t.Errorf("minLon >= maxLon: %.6f >= %.6f", bounds[0], bounds[2])
	}
	if bounds[1] >= bounds[3] {
		t.Errorf("minLat >= maxLat: %.6f >= %.6f", bounds[1], bounds[3])
	}
}

func TestCoordsBoundsMercator(t *testing.T) {
	coords := Coords{Z: 13, X: 4297, Y: 2754}
	mercBounds := coords.BoundsMercator()

	t.Logf("Tile %s Mercator bounds: [%.2f, %.2f, %.2f, %.2f]",
		coords.String(), mercBounds[0], mercBounds[1], mercBounds[2], mercBounds[3])

	// Verify bounds are ordered correctly
	if mercBounds[0] >= mercBounds[2] {
		t.Errorf("minX >= maxX: %.2f >= %.2f", mercBounds[0], mercBounds[2])
	}
	if mercBounds[1] >= mercBounds[3] {
		t.Errorf("minY >= maxY: %.2f >= %.2f", mercBounds[1], mercBounds[3])
	}

	// Web Mercator bounds should be in reasonable range
	// (roughly -20037508 to 20037508 meters)
	for i, val := range mercBounds {
		if math.Abs(val) > 20037508 {
			t.Errorf("Mercator coordinate[%d] = %.2f is out of valid range", i, val)
		}
	}
}

func TestCoordsCenter(t *testing.T) {
	coords := Coords{Z: 13, X: 4297, Y: 2754}
	lon, lat := coords.Center()

	t.Logf("Tile %s center: %.6f, %.6f", coords.String(), lon, lat)

	// Verify center is within bounds
	bounds := coords.Bounds()
	if lon < bounds[0] || lon > bounds[2] {
		t.Errorf("Center lon %.6f is outside bounds [%.6f, %.6f]", lon, bounds[0], bounds[2])
	}
	if lat < bounds[1] || lat > bounds[3] {
		t.Errorf("Center lat %.6f is outside bounds [%.6f, %.6f]", lat, bounds[1], bounds[3])
	}
}

func TestParseCoords(t *testing.T) {
	tests := []struct {
		input    string
		expected Coords
		wantErr  bool
	}{
		{"z13_x4297_y2754", Coords{Z: 13, X: 4297, Y: 2754}, false},
		{"z0_x0_y0", Coords{Z: 0, X: 0, Y: 0}, false},
		{"z18_x262143_y262143", Coords{Z: 18, X: 262143, Y: 262143}, false},
		{"invalid", Coords{}, true},
		{"z13_x4297", Coords{}, true},
		{"13_4297_2754", Coords{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseCoords(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCoords(%s) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseCoords(%s) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseCoords(%s) = %+v, want %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMercatorConversion(t *testing.T) {
	// Test round-trip conversion
	testPoints := [][2]float64{
		{0, 0},           // Null Island
		{9.73, 52.37},    // Hanover
		{-122.42, 37.78}, // San Francisco
		{139.69, 35.69},  // Tokyo
	}

	for _, point := range testPoints {
		lon, lat := point[0], point[1]

		// Convert to Mercator and back
		x, y := lonLatToMercator(lon, lat)
		lon2, lat2 := mercatorToLonLat(x, y)

		// Check round-trip accuracy (should be very close)
		lonDiff := math.Abs(lon - lon2)
		latDiff := math.Abs(lat - lat2)

		t.Logf("Point (%.2f, %.2f) -> Mercator (%.2f, %.2f) -> (%.6f, %.6f)",
			lon, lat, x, y, lon2, lat2)

		if lonDiff > 0.000001 || latDiff > 0.000001 {
			t.Errorf("Round-trip conversion failed: (%.6f, %.6f) != (%.6f, %.6f)",
				lon, lat, lon2, lat2)
		}
	}
}

func TestTileRange(t *testing.T) {
	// Test range covering a few tiles
	tr := TileRange{
		MinZ: 13, MaxZ: 13,
		MinX: 4297, MaxX: 4298,
		MinY: 2754, MaxY: 2755,
	}

	// Should have 4 tiles (2x2)
	expectedCount := 4
	if tr.Count() != expectedCount {
		t.Errorf("Count() = %d, want %d", tr.Count(), expectedCount)
	}

	// Test ForEach
	var visited []string
	tr.ForEach(func(c Coords) {
		visited = append(visited, c.String())
	})

	if len(visited) != expectedCount {
		t.Errorf("ForEach visited %d tiles, want %d", len(visited), expectedCount)
	}

	t.Logf("Visited tiles: %v", visited)
}

func TestTileRangeFromBounds(t *testing.T) {
	// Use the bounds from our test tile
	testTile := Coords{Z: 13, X: 4297, Y: 2754}
	bounds := testTile.Bounds()

	t.Logf("Test tile %s bounds: [%.6f, %.6f, %.6f, %.6f]",
		testTile.String(), bounds[0], bounds[1], bounds[2], bounds[3])

	tr := TileRangeFromBounds(13, 13, bounds)

	t.Logf("Tile range: z%d x[%d-%d] y[%d-%d]",
		tr.MinZ, tr.MinX, tr.MaxX, tr.MinY, tr.MaxY)
	t.Logf("Total tiles: %d", tr.Count())

	// Range should have at least one tile
	if tr.Count() == 0 {
		t.Errorf("Expected at least one tile in range, got 0")
	}

	// Verify the range makes sense
	if tr.MinX > tr.MaxX || tr.MinY > tr.MaxY {
		t.Errorf("Invalid tile range: x[%d-%d] y[%d-%d]", tr.MinX, tr.MaxX, tr.MinY, tr.MaxY)
	}
}
