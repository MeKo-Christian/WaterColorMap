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

	return &types.TileData{
		Coordinate: tile,
		Bounds:     bounds,
		Features:   features,
		FetchedAt:  time.Now(),
		Source:     "overpass-api",
	}, nil
}

// buildTileQuery creates a comprehensive Overpass QL query for tile features.
// It fetches complete geometry for all ways that intersect the bounding box,
// not just the portions within the bbox. This prevents polygon clipping artifacts
// at tile boundaries.
func (ds *OverpassDataSource) buildTileQuery(bounds types.BoundingBox) string {
	// Build query with all feature types we need.
	// IMPORTANT: We use per-element bbox filters (south,west,north,east) instead of
	// the global [bbox:] setting. When using per-element bbox filters with "out geom",
	// Overpass returns the COMPLETE geometry of ways that intersect the bbox,
	// rather than clipping geometry to the bbox boundary.
	bbox := fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", bounds.MinLat, bounds.MinLon, bounds.MaxLat, bounds.MaxLon)
	return fmt.Sprintf(`
[out:json][timeout:60];
(
  way["natural"="water"](%s);
  way["natural"="coastline"](%s);
  way["waterway"](%s);
  relation["natural"="water"](%s);
  relation["waterway"](%s);
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
out geom;
`, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox, bbox)
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
