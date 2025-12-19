package datasource

import (
	"encoding/json"
	"fmt"

	"github.com/MeKo-Christian/go-overpass"
	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/paulmach/orb"
)

// UnmarshalOverpassJSON decodes an Overpass API JSON response into an overpass.Result.
// This is used by the WASM playground (browser fetch + Go-side parsing).
func UnmarshalOverpassJSON(data []byte) (*overpass.Result, error) {
	var result overpass.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overpass json: %w", err)
	}
	return &result, nil
}

// ExtractFeaturesFromOverpassResult converts an Overpass result to WaterColorMap's FeatureCollection.
// It mirrors the logic used by OverpassDataSource.
func ExtractFeaturesFromOverpassResult(result *overpass.Result) types.FeatureCollection {
	var features types.FeatureCollection
	if result == nil {
		return features
	}

	// Process ways
	for _, way := range result.Ways {
		feature := convertWayToFeature(way)
		if feature == nil {
			continue
		}

		switch {
		case isWater(way.Tags):
			features.Water = append(features.Water, *feature)
		case isPark(way.Tags):
			features.Parks = append(features.Parks, *feature)
		case isRoad(way.Tags):
			features.Roads = append(features.Roads, *feature)
		case isBuilding(way.Tags):
			features.Buildings = append(features.Buildings, *feature)
		case isCivic(way.Tags):
			features.Civic = append(features.Civic, *feature)
		}
	}

	// Process relations (mainly for multipolygon water bodies and parks)
	for _, rel := range result.Relations {
		feature := convertRelationToFeature(rel)
		if feature == nil {
			continue
		}

		switch {
		case isWater(rel.Tags):
			features.Water = append(features.Water, *feature)
		case isPark(rel.Tags):
			features.Parks = append(features.Parks, *feature)
		}
	}

	return features
}

func convertWayToFeature(way *overpass.Way) *types.Feature {
	if way == nil || len(way.Geometry) == 0 {
		return nil
	}

	var geometry orb.Geometry
	points := make(orb.LineString, len(way.Geometry))

	for i, point := range way.Geometry {
		points[i] = orb.Point{point.Lon, point.Lat}
	}

	if len(points) > 2 && points[0] == points[len(points)-1] {
		ring := orb.Ring(points)
		geometry = orb.Polygon{ring}
	} else {
		geometry = points
	}

	name := ""
	if n, ok := way.Tags["name"]; ok {
		name = n
	}

	featureType := categorizeByTags(way.Tags)

	return &types.Feature{
		ID:         fmt.Sprintf("way/%d", way.ID),
		Type:       featureType,
		Geometry:   geometry,
		Properties: convertTags(way.Tags),
		Name:       name,
	}
}

func convertRelationToFeature(rel *overpass.Relation) *types.Feature {
	if rel == nil {
		return nil
	}

	name := ""
	if n, ok := rel.Tags["name"]; ok {
		name = n
	}

	featureType := categorizeByTags(rel.Tags)

	return &types.Feature{
		ID:         fmt.Sprintf("relation/%d", rel.ID),
		Type:       featureType,
		Geometry:   orb.Point{},
		Properties: convertTags(rel.Tags),
		Name:       name,
	}
}

func categorizeByTags(tags map[string]string) types.FeatureType {
	if isWater(tags) {
		return types.FeatureTypeWater
	}
	if isPark(tags) {
		return types.FeatureTypePark
	}
	if isRoad(tags) {
		return types.FeatureTypeRoad
	}
	if isBuilding(tags) {
		return types.FeatureTypeBuilding
	}
	if isCivic(tags) {
		return types.FeatureTypeCivic
	}
	return types.FeatureTypeUnknown
}

func isWater(tags map[string]string) bool {
	return tags["natural"] == "water" ||
		tags["natural"] == "coastline" ||
		tags["waterway"] != ""
}

func isPark(tags map[string]string) bool {
	return tags["leisure"] == "park" ||
		tags["leisure"] == "garden" ||
		tags["landuse"] == "forest" ||
		tags["landuse"] == "grass" ||
		tags["landuse"] == "meadow"
}

func isRoad(tags map[string]string) bool {
	return tags["highway"] != ""
}

func isBuilding(tags map[string]string) bool {
	return tags["building"] != ""
}

func isCivic(tags map[string]string) bool {
	amenity := tags["amenity"]
	return amenity == "school" ||
		amenity == "hospital" ||
		amenity == "university" ||
		amenity == "library" ||
		amenity == "town_hall"
}
