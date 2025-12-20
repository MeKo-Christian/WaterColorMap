package types

import (
	"time"

	"github.com/MeKo-Christian/go-overpass"
	"github.com/paulmach/orb"
)

// FeatureType represents the type of geographic feature
type FeatureType string

const (
	FeatureTypeWater    FeatureType = "water"
	FeatureTypePark     FeatureType = "park"
	FeatureTypeRoad     FeatureType = "road"
	FeatureTypeBuilding FeatureType = "building"
	FeatureTypeCivic    FeatureType = "civic"
	FeatureTypeLand     FeatureType = "land"
	FeatureTypeUnknown  FeatureType = "unknown"
)

// Feature represents a geographic feature extracted from OSM
type Feature struct {
	ID         string                 // OSM element ID (e.g., "way/12345")
	Type       FeatureType            // Feature category
	Geometry   orb.Geometry           // Geometry (Point, LineString, Polygon, MultiPolygon)
	Properties map[string]interface{} // OSM tags and additional properties
	Name       string                 // Feature name (if available)
}

// FeatureCollection groups features by type
type FeatureCollection struct {
	Water     []Feature // Lakes, rivers, coastlines
	Parks     []Feature // Parks, forests, green spaces
	Roads     []Feature // Streets, highways
	Buildings []Feature // Building footprints
	Civic     []Feature // Schools, hospitals, landmarks
	Land      []Feature // Land polygons (background)
}

// TileData represents all data for a single tile
type TileData struct {
	FetchedAt  time.Time
	Source     string
	Features   FeatureCollection
	Bounds     BoundingBox
	Coordinate TileCoordinate

	// OverpassResult stores the raw Overpass API response for debugging purposes.
	// This is nil by default to save memory in production. To enable, call
	// WithRawResponseStorage(true) on the OverpassDataSource.
	// Only use in tests or debugging scenarios.
	OverpassResult *overpass.Result
}

// Count returns the total number of features
func (fc FeatureCollection) Count() int {
	return len(fc.Water) + len(fc.Parks) + len(fc.Roads) + len(fc.Buildings) + len(fc.Civic) + len(fc.Land)
}

// FeatureCounts returns a map of feature counts by type
func (fc FeatureCollection) FeatureCounts() map[string]int {
	return map[string]int{
		"water":     len(fc.Water),
		"parks":     len(fc.Parks),
		"roads":     len(fc.Roads),
		"buildings": len(fc.Buildings),
		"civic":     len(fc.Civic),
		"land":      len(fc.Land),
		"total":     fc.Count(),
	}
}
