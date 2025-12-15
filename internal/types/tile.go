package types

import (
	"fmt"
	"math"
)

// TileCoordinate represents a tile in the Web Mercator tile system
type TileCoordinate struct {
	Zoom int // Zoom level (0-18)
	X    int // Tile column (0 to 2^zoom - 1, west to east)
	Y    int // Tile row (0 to 2^zoom - 1, north to south)
}

// BoundingBox represents a geographic bounding box in WGS84 (EPSG:4326)
type BoundingBox struct {
	MinLon float64 // Western edge (degrees)
	MinLat float64 // Southern edge (degrees)
	MaxLon float64 // Eastern edge (degrees)
	MaxLat float64 // Northern edge (degrees)
}

// TileToBounds converts tile coordinates to geographic bounding box
func TileToBounds(coord TileCoordinate) BoundingBox {
	n := math.Pow(2, float64(coord.Zoom))

	minLon := float64(coord.X)/n*360.0 - 180.0
	maxLon := float64(coord.X+1)/n*360.0 - 180.0

	minLat := mercatorToLat(math.Pi * (1 - 2*float64(coord.Y+1)/n))
	maxLat := mercatorToLat(math.Pi * (1 - 2*float64(coord.Y)/n))

	return BoundingBox{
		MinLon: minLon,
		MinLat: minLat,
		MaxLon: maxLon,
		MaxLat: maxLat,
	}
}

// mercatorToLat converts Web Mercator Y coordinate to latitude
func mercatorToLat(mercatorY float64) float64 {
	return 180.0 / math.Pi * math.Atan(math.Sinh(mercatorY))
}

// String returns a human-readable representation of the tile coordinate
func (t TileCoordinate) String() string {
	return fmt.Sprintf("z%d_x%d_y%d", t.Zoom, t.X, t.Y)
}

// Filename returns the standard filename for this tile
func (t TileCoordinate) Filename() string {
	return fmt.Sprintf("z%d_x%d_y%d.png", t.Zoom, t.X, t.Y)
}

// String returns a human-readable representation of the bounding box
func (b BoundingBox) String() string {
	return fmt.Sprintf("bbox(%.6f,%.6f,%.6f,%.6f)", b.MinLat, b.MinLon, b.MaxLat, b.MaxLon)
}

// Center returns the center point of the bounding box
func (b BoundingBox) Center() (lat, lon float64) {
	return (b.MinLat + b.MaxLat) / 2, (b.MinLon + b.MaxLon) / 2
}

// Width returns the width of the bounding box in degrees
func (b BoundingBox) Width() float64 {
	return b.MaxLon - b.MinLon
}

// Height returns the height of the bounding box in degrees
func (b BoundingBox) Height() float64 {
	return b.MaxLat - b.MinLat
}
