package geojson

import (
	"encoding/json"
	"fmt"

	"github.com/MeKo-Tech/watercolormap/internal/types"
	"github.com/paulmach/orb/geojson"
)

// LayerType represents the different map layers we render
type LayerType string

const (
	LayerWater    LayerType = "water"
	LayerLand     LayerType = "land"
	LayerParks    LayerType = "parks"
	LayerCivic    LayerType = "civic"
	LayerRoads    LayerType = "roads"
	LayerHighways LayerType = "highways"
	LayerPaper    LayerType = "paper"
)

// ToGeoJSON converts a slice of features to GeoJSON FeatureCollection
func ToGeoJSON(features []types.Feature) (*geojson.FeatureCollection, error) {
	fc := geojson.NewFeatureCollection()

	for _, f := range features {
		if f.Geometry == nil {
			continue
		}

		// Create GeoJSON feature
		geoFeature := geojson.NewFeature(f.Geometry)

		// Add properties
		if geoFeature.Properties == nil {
			geoFeature.Properties = make(map[string]interface{})
		}

		// Copy all properties from the feature
		for key, value := range f.Properties {
			geoFeature.Properties[key] = value
		}

		// Add OSM ID and name
		geoFeature.Properties["osm_id"] = f.ID
		if f.Name != "" {
			geoFeature.Properties["name"] = f.Name
		}

		// Add feature type
		geoFeature.Properties["feature_type"] = string(f.Type)

		fc.Append(geoFeature)
	}

	return fc, nil
}

// ToGeoJSONBytes converts features to GeoJSON bytes
func ToGeoJSONBytes(features []types.Feature) ([]byte, error) {
	fc, err := ToGeoJSON(features)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to GeoJSON: %w", err)
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GeoJSON: %w", err)
	}

	return data, nil
}

// GetLayerFeatures returns features for a specific layer from FeatureCollection
func GetLayerFeatures(fc types.FeatureCollection, layer LayerType) []types.Feature {
	switch layer {
	case LayerWater:
		return fc.Water
	case LayerParks:
		return fc.Parks
	case LayerCivic:
		// Combine civic and buildings for the civic layer
		combined := make([]types.Feature, 0, len(fc.Civic)+len(fc.Buildings))
		combined = append(combined, fc.Civic...)
		combined = append(combined, fc.Buildings...)
		return combined
	case LayerRoads:
		return fc.Roads
	case LayerHighways:
		// Highways/major roads are derived from the generic roads feature set.
		// We keep this as a view rather than adding a separate collection bucket.
		out := make([]types.Feature, 0, len(fc.Roads))
		for _, f := range fc.Roads {
			hw, _ := f.Properties["highway"].(string)
			switch hw {
			case "motorway", "motorway_link", "trunk", "trunk_link", "primary", "primary_link", "secondary", "secondary_link":
				out = append(out, f)
			}
		}
		return out
	case LayerLand:
		return fc.Land
	default:
		return nil
	}
}

// LayerCount returns the number of features in a layer
func LayerCount(fc types.FeatureCollection, layer LayerType) int {
	return len(GetLayerFeatures(fc, layer))
}

// LayerSummary returns a summary of features per layer
func LayerSummary(fc types.FeatureCollection) string {
	return fmt.Sprintf("Water: %d, Parks: %d, Civic: %d, Buildings: %d, Roads: %d (Total: %d)",
		len(fc.Water), len(fc.Parks), len(fc.Civic), len(fc.Buildings), len(fc.Roads), fc.Count())
}
