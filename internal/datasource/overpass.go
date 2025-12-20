package datasource

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/MeKo-Christian/go-overpass"
	"github.com/MeKo-Tech/watercolormap/internal/types"
)

// OverpassDataSource fetches OSM data from Overpass API
type OverpassDataSource struct {
	client           overpass.Client
	storeRawResponse bool // If true, stores raw Overpass response in TileData (for debugging)
	clipGeomToBbox   bool // If true, uses "out geom(bbox)" - DO NOT USE (known Overpass API bug)
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
		client:           client,
		storeRawResponse: false, // Don't store raw response by default (saves memory)
		clipGeomToBbox:   false, // Don't clip geometry (prevents artifacts from Overpass bug)
	}
}

// WithRawResponseStorage enables storing the raw Overpass API response in TileData.
// This is useful for debugging but increases memory usage. Should only be used in tests.
func (ds *OverpassDataSource) WithRawResponseStorage(enabled bool) *OverpassDataSource {
	ds.storeRawResponse = enabled
	return ds
}

// WithGeometryClipping enables clipping geometry to bbox in Overpass query.
//
// WARNING: DO NOT USE IN PRODUCTION. This has a known Overpass API bug.
//
// When enabled, uses "out geom(bbox)" which should clip geometry to the bbox boundary.
// However, the Overpass API has a known regression (https://github.com/drolbr/Overpass-API/issues/417)
// where this returns malformed/wrapped geometry for ways not fully contained in the bbox.
// Visual testing confirmed severe rendering artifacts (distorted/wrapped polygons).
//
// This method is kept for potential future use if the Overpass API bug is fixed.
// Default is disabled (false).
func (ds *OverpassDataSource) WithGeometryClipping(enabled bool) *OverpassDataSource {
	ds.clipGeomToBbox = enabled
	return ds
}

// FetchTileData fetches all OSM features for a tile
func (ds *OverpassDataSource) FetchTileData(ctx context.Context, tile types.TileCoordinate) (*types.TileData, error) {
	return ds.FetchTileDataWithBounds(ctx, tile, types.TileToBounds(tile))
}

// FetchTileDataWithBounds fetches OSM features using an explicit bounding box.
// This is useful for "metatile" rendering where we need data slightly outside
// the tile bounds (e.g. to support post-processing blurs without seams).
func (ds *OverpassDataSource) FetchTileDataWithBounds(ctx context.Context, tile types.TileCoordinate, bounds types.BoundingBox) (*types.TileData, error) {
	// Build Overpass QL query manually
	query := ds.buildTileQuery(bounds)

	// Execute query (note: this version doesn't support context)
	result, err := ds.client.Query(query)
	if err != nil {
		return nil, fmt.Errorf("overpass query failed: %w", err)
	}

	// Convert to feature collection
	features := ExtractFeaturesFromOverpassResult(&result)

	tileData := &types.TileData{
		Coordinate: tile,
		Bounds:     bounds,
		Features:   features,
		FetchedAt:  time.Now(),
		Source:     "overpass-api",
	}

	// Only store raw response if explicitly requested (for debugging/tests)
	if ds.storeRawResponse {
		tileData.OverpassResult = &result
	}

	return tileData, nil
}

// buildTileQuery creates a comprehensive Overpass QL query for tile features.
// It fetches COMPLETE unclipped geometry for all ways that intersect the bounding box.
//
// IMPORTANT: Uses "out geom qt;" to return COMPLETE geometry (not clipped to bbox).
// This prevents polygon clipping artifacts at tile boundaries.
//
// WARNING: The clipGeomToBbox option is available but should NOT be used due to a known
// Overpass API bug (https://github.com/drolbr/Overpass-API/issues/417) where "out geom(bbox)"
// returns malformed/wrapped geometry for partially-included ways. Visual testing confirmed
// severe rendering artifacts (distorted/wrapped polygons). Only use if the Overpass bug is fixed.
//
// Output modifiers:
// - "geom" returns complete geometry for ways intersecting the bbox
// - "geom(bbox)" clips geometry to bbox (BROKEN - causes malformed geometry)
// - "qt" (quiet) omits metadata (version, changeset, timestamp, user, uid)
func (ds *OverpassDataSource) buildTileQuery(bounds types.BoundingBox) string {
	// Build query with all feature types we need.
	// IMPORTANT: We use per-element bbox filters (south,west,north,east) instead of
	// the global [bbox:] setting. When using per-element bbox filters with "out geom",
	// Overpass returns the COMPLETE geometry of ways that intersect the bbox,
	// rather than clipping geometry to the bbox boundary.
	bbox := fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", bounds.MinLat, bounds.MinLon, bounds.MaxLat, bounds.MaxLon)

	// Choose output mode based on clipping setting
	var outputMode string
	if ds.clipGeomToBbox {
		// WARNING: This produces malformed geometry due to Overpass API bug
		outputMode = fmt.Sprintf("out geom(%s) qt;", bbox)
	} else {
		outputMode = "out geom qt;"
	}

	return fmt.Sprintf(`
[out:json][timeout:60];
(
  way["natural"="water"](%s);
  way["natural"="coastline"](%s);
  relation["natural"="water"](%s);
  way["leisure"="park"](%s);
  way["leisure"="garden"](%s);
  way["landuse"="forest"](%s);
  way["landuse"="grass"](%s);
  way["landuse"="meadow"](%s);
  relation["leisure"="park"](%s);
  way["highway"](%s);
  way["building"](%s);
  way["amenity"="school"](%s);
  way["amenity"="hospital"](%s);
  way["amenity"="university"](%s);
);
%s
`, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, outputMode)
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
