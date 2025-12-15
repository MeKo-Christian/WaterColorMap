package datasource

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/paulmach/orb"
	"github.com/serjvanilla/go-overpass"
)

// OverpassDataSource fetches OSM data from Overpass API
type OverpassDataSource struct {
	client overpass.Client
}

// NewOverpassDataSource creates a new Overpass data source
func NewOverpassDataSource(endpoint string) *OverpassDataSource {
	if endpoint == "" {
		endpoint = "https://overpass-api.de/api/interpreter"
	}

	// Create client (rate limited to 1 concurrent request)
	client := overpass.NewWithSettings(
		endpoint,
		1, // Only 1 parallel request (API etiquette)
		http.DefaultClient,
	)

	return &OverpassDataSource{
		client: client,
	}
}

// FetchTileData fetches all OSM features for a tile
func (ds *OverpassDataSource) FetchTileData(ctx context.Context, tile types.TileCoordinate) (*types.TileData, error) {
	// Calculate bounding box for tile
	bounds := types.TileToBounds(tile)

	// Build Overpass QL query manually
	query := ds.buildTileQuery(bounds)

	// Execute query (note: this version doesn't support context)
	result, err := ds.client.Query(query)
	if err != nil {
		return nil, fmt.Errorf("overpass query failed: %w", err)
	}

	// Convert to feature collection
	features := ds.extractFeatures(&result)

	return &types.TileData{
		Coordinate: tile,
		Bounds:     bounds,
		Features:   features,
		FetchedAt:  time.Now(),
		Source:     "overpass-api",
	}, nil
}

// buildTileQuery creates a comprehensive Overpass QL query for tile features
func (ds *OverpassDataSource) buildTileQuery(bounds types.BoundingBox) string {
	// Build query with all feature types we need
	// Using union operator to get multiple feature types in one query
	return fmt.Sprintf(`
[out:json][timeout:30][bbox:%.6f,%.6f,%.6f,%.6f];
(
  way["natural"="water"];
  way["natural"="coastline"];
  way["waterway"];
  relation["natural"="water"];
  relation["waterway"];
  way["leisure"="park"];
  way["leisure"="garden"];
  way["landuse"="forest"];
  way["landuse"="grass"];
  way["landuse"="meadow"];
  relation["leisure"="park"];
  way["highway"];
  way["building"];
  way["amenity"="school"];
  way["amenity"="hospital"];
  way["amenity"="university"];
);
out geom;
`, bounds.MinLat, bounds.MinLon, bounds.MaxLat, bounds.MaxLon)
}

// extractFeatures converts Overpass result to feature collection
func (ds *OverpassDataSource) extractFeatures(result *overpass.Result) types.FeatureCollection {
	var features types.FeatureCollection

	// Process ways
	for _, way := range result.Ways {
		feature := ds.convertWayToFeature(way)
		if feature == nil {
			continue
		}

		// Categorize using tag checking
		switch {
		case ds.isWater(way.Tags):
			features.Water = append(features.Water, *feature)
		case ds.isPark(way.Tags):
			features.Parks = append(features.Parks, *feature)
		case ds.isRoad(way.Tags):
			features.Roads = append(features.Roads, *feature)
		case ds.isBuilding(way.Tags):
			features.Buildings = append(features.Buildings, *feature)
		case ds.isCivic(way.Tags):
			features.Civic = append(features.Civic, *feature)
		}
	}

	// Process relations (mainly for multipolygon water bodies and parks)
	for _, rel := range result.Relations {
		feature := ds.convertRelationToFeature(rel)
		if feature == nil {
			continue
		}

		// Categorize relations
		switch {
		case ds.isWater(rel.Tags):
			features.Water = append(features.Water, *feature)
		case ds.isPark(rel.Tags):
			features.Parks = append(features.Parks, *feature)
		}
	}

	return features
}

// convertWayToFeature converts an OSM way to a Feature
func (ds *OverpassDataSource) convertWayToFeature(way *overpass.Way) *types.Feature {
	if way == nil || len(way.Geometry) == 0 {
		return nil
	}

	// Convert coordinates to orb.LineString or orb.Polygon
	var geometry orb.Geometry
	points := make(orb.LineString, len(way.Geometry))

	for i, point := range way.Geometry {
		points[i] = orb.Point{point.Lon, point.Lat}
	}

	// Check if way is closed (polygon)
	if len(points) > 2 && points[0] == points[len(points)-1] {
		// Closed way = polygon
		ring := orb.Ring(points)
		geometry = orb.Polygon{ring}
	} else {
		// Open way = linestring
		geometry = points
	}

	// Extract name
	name := ""
	if n, ok := way.Tags["name"]; ok {
		name = n
	}

	// Determine feature type
	featureType := ds.categorizeByTags(way.Tags)

	return &types.Feature{
		ID:         fmt.Sprintf("way/%d", way.ID),
		Type:       featureType,
		Geometry:   geometry,
		Properties: convertTags(way.Tags),
		Name:       name,
	}
}

// convertRelationToFeature converts an OSM relation to a Feature
func (ds *OverpassDataSource) convertRelationToFeature(rel *overpass.Relation) *types.Feature {
	if rel == nil {
		return nil
	}

	// For now, we'll skip complex relation geometry parsing
	// In a full implementation, we'd assemble multipolygons from members
	// This is a simplified version that just marks the relation exists

	name := ""
	if n, ok := rel.Tags["name"]; ok {
		name = n
	}

	featureType := ds.categorizeByTags(rel.Tags)

	return &types.Feature{
		ID:         fmt.Sprintf("relation/%d", rel.ID),
		Type:       featureType,
		Geometry:   orb.Point{}, // Placeholder - would need full geometry assembly
		Properties: convertTags(rel.Tags),
		Name:       name,
	}
}

// categorizeByTags determines the feature type from OSM tags
func (ds *OverpassDataSource) categorizeByTags(tags map[string]string) types.FeatureType {
	if ds.isWater(tags) {
		return types.FeatureTypeWater
	}
	if ds.isPark(tags) {
		return types.FeatureTypePark
	}
	if ds.isRoad(tags) {
		return types.FeatureTypeRoad
	}
	if ds.isBuilding(tags) {
		return types.FeatureTypeBuilding
	}
	if ds.isCivic(tags) {
		return types.FeatureTypeCivic
	}
	return types.FeatureTypeUnknown
}

// isWater checks if tags indicate a water feature
func (ds *OverpassDataSource) isWater(tags map[string]string) bool {
	return tags["natural"] == "water" ||
		tags["natural"] == "coastline" ||
		tags["waterway"] != ""
}

// isPark checks if tags indicate a park or green space
func (ds *OverpassDataSource) isPark(tags map[string]string) bool {
	return tags["leisure"] == "park" ||
		tags["leisure"] == "garden" ||
		tags["landuse"] == "forest" ||
		tags["landuse"] == "grass" ||
		tags["landuse"] == "meadow"
}

// isRoad checks if tags indicate a road or highway
func (ds *OverpassDataSource) isRoad(tags map[string]string) bool {
	return tags["highway"] != ""
}

// isBuilding checks if tags indicate a building
func (ds *OverpassDataSource) isBuilding(tags map[string]string) bool {
	return tags["building"] != ""
}

// isCivic checks if tags indicate a civic building
func (ds *OverpassDataSource) isCivic(tags map[string]string) bool {
	amenity := tags["amenity"]
	return amenity == "school" ||
		amenity == "hospital" ||
		amenity == "university" ||
		amenity == "library" ||
		amenity == "town_hall"
}

// convertTags converts OSM tags to generic properties map
func convertTags(tags map[string]string) map[string]interface{} {
	props := make(map[string]interface{}, len(tags))
	for k, v := range tags {
		props[k] = v
	}
	return props
}

// Close cleans up resources (no-op for current version)
func (ds *OverpassDataSource) Close() error {
	return nil
}

// ClearCache is a no-op for current version (no cache support)
func (ds *OverpassDataSource) ClearCache() {
	// No cache in current version
}

// CacheSize returns 0 (no cache in current version)
func (ds *OverpassDataSource) CacheSize() int {
	return 0
}
