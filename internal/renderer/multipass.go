package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MeKo-Tech/watercolormap/internal/geojson"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/MeKo-Tech/watercolormap/internal/types"
)

// MultiPassRenderer renders tiles in multiple passes, one per layer
type MultiPassRenderer struct {
	mapnikRenderer *MapnikRenderer
	stylesDir      string
	outputDir      string
	tempDir        string
}

// LayerRenderResult contains the result of rendering a single layer
type LayerRenderResult struct {
	Layer      geojson.LayerType
	OutputPath string
	Error      error
}

// TileRenderResult contains the results of rendering all layers for a tile
type TileRenderResult struct {
	TileCoords tile.Coords
	Layers     map[geojson.LayerType]*LayerRenderResult
	TotalTime  float64 // seconds
}

// NewMultiPassRenderer creates a new multi-pass renderer
func NewMultiPassRenderer(stylesDir, outputDir string) (*MultiPassRenderer, error) {
	// Create Mapnik renderer (empty style file, 256x256 tile size)
	mapnikRenderer, err := NewMapnikRenderer("", 256)
	if err != nil {
		return nil, fmt.Errorf("failed to create Mapnik renderer: %w", err)
	}

	// Create temp directory for GeoJSON files
	tempDir := filepath.Join(os.TempDir(), "watercolormap")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &MultiPassRenderer{
		mapnikRenderer: mapnikRenderer,
		stylesDir:      stylesDir,
		outputDir:      outputDir,
		tempDir:        tempDir,
	}, nil
}

// Close cleans up resources
func (r *MultiPassRenderer) Close() error {
	return r.mapnikRenderer.Close()
}

// RenderTile renders all layers for a single tile
func (r *MultiPassRenderer) RenderTile(coords tile.Coords, data *types.TileData) (*TileRenderResult, error) {
	result := &TileRenderResult{
		TileCoords: coords,
		Layers:     make(map[geojson.LayerType]*LayerRenderResult),
	}

	// Define the layers to render in order
	layers := []geojson.LayerType{
		geojson.LayerLand,  // Background layer (just background color)
		geojson.LayerWater, // Water bodies
		geojson.LayerParks, // Parks and green spaces
		geojson.LayerCivic, // Buildings and civic areas
		geojson.LayerRoads, // Roads
	}

	// Get bounds for the tile
	bounds := coords.BoundsMercator()

	// Render each layer
	for _, layer := range layers {
		layerResult := r.renderLayer(coords, layer, data, bounds)
		result.Layers[layer] = layerResult

		if layerResult.Error != nil {
			// Log error but continue with other layers
			fmt.Printf("Warning: Failed to render layer %s for tile %s: %v\n",
				layer, coords.String(), layerResult.Error)
		}
	}

	return result, nil
}

// renderLayer renders a single layer
func (r *MultiPassRenderer) renderLayer(
	coords tile.Coords,
	layer geojson.LayerType,
	data *types.TileData,
	bounds [4]float64,
) *LayerRenderResult {
	result := &LayerRenderResult{
		Layer: layer,
	}

	// Get style file path
	stylePath := filepath.Join(r.stylesDir, "layers", fmt.Sprintf("%s.xml", layer))
	if _, err := os.Stat(stylePath); err != nil {
		result.Error = fmt.Errorf("style file not found: %s", stylePath)
		return result
	}

	// Special case: land layer (no features, just background)
	if layer == geojson.LayerLand {
		return r.renderLandLayer(coords, stylePath, bounds)
	}

	// Get features for this layer
	features := geojson.GetLayerFeatures(data.Features, layer)
	if len(features) == 0 {
		// No features for this layer - skip rendering
		result.OutputPath = ""
		return result
	}

	// Convert features to GeoJSON
	geoJSONBytes, err := geojson.ToGeoJSONBytes(features)
	if err != nil {
		result.Error = fmt.Errorf("failed to convert to GeoJSON: %w", err)
		return result
	}

	// Write GeoJSON to temporary file
	geoJSONPath := filepath.Join(r.tempDir, fmt.Sprintf("%s_%s.geojson", coords.String(), layer))
	if err := os.WriteFile(geoJSONPath, geoJSONBytes, 0644); err != nil {
		result.Error = fmt.Errorf("failed to write GeoJSON: %w", err)
		return result
	}
	defer os.Remove(geoJSONPath) // Clean up temp file

	// Load style XML and replace datasource placeholder
	styleXML, err := os.ReadFile(stylePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read style file: %w", err)
		return result
	}

	// Replace DATASOURCE_PLACEHOLDER with actual GeoJSON path
	modifiedStyleXML := strings.ReplaceAll(string(styleXML), "DATASOURCE_PLACEHOLDER", geoJSONPath)

	// Load style into Mapnik
	if err := r.mapnikRenderer.LoadXML(modifiedStyleXML); err != nil {
		result.Error = fmt.Errorf("failed to load style: %w", err)
		return result
	}

	// Set map bounds
	if err := r.mapnikRenderer.SetBounds(bounds[0], bounds[1], bounds[2], bounds[3]); err != nil {
		result.Error = fmt.Errorf("failed to set bounds: %w", err)
		return result
	}

	// Render to file
	outputPath := filepath.Join(r.outputDir, fmt.Sprintf("%s_%s.png", coords.String(), layer))

	// Create a dummy TileCoordinate for compatibility with RenderToFile
	dummyTile := types.TileCoordinate{
		Zoom: int(coords.Z),
		X:    int(coords.X),
		Y:    int(coords.Y),
	}

	if err := r.mapnikRenderer.RenderToFile(dummyTile, outputPath); err != nil {
		result.Error = fmt.Errorf("failed to render: %w", err)
		return result
	}

	result.OutputPath = outputPath
	return result
}

// renderLandLayer renders the land layer (just background color, no features)
func (r *MultiPassRenderer) renderLandLayer(
	coords tile.Coords,
	stylePath string,
	bounds [4]float64,
) *LayerRenderResult {
	result := &LayerRenderResult{
		Layer: geojson.LayerLand,
	}

	// Load style XML (land style has background color, no datasource)
	styleXML, err := os.ReadFile(stylePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read land style: %w", err)
		return result
	}

	// Load style into Mapnik
	if err := r.mapnikRenderer.LoadXML(string(styleXML)); err != nil {
		result.Error = fmt.Errorf("failed to load land style: %w", err)
		return result
	}

	// Set map bounds
	if err := r.mapnikRenderer.SetBounds(bounds[0], bounds[1], bounds[2], bounds[3]); err != nil {
		result.Error = fmt.Errorf("failed to set bounds: %w", err)
		return result
	}

	// Render to file
	outputPath := filepath.Join(r.outputDir, fmt.Sprintf("%s_%s.png", coords.String(), geojson.LayerLand))

	// Create a dummy TileCoordinate for compatibility with RenderToFile
	dummyTile := types.TileCoordinate{
		Zoom: int(coords.Z),
		X:    int(coords.X),
		Y:    int(coords.Y),
	}

	if err := r.mapnikRenderer.RenderToFile(dummyTile, outputPath); err != nil {
		result.Error = fmt.Errorf("failed to render land layer: %w", err)
		return result
	}

	result.OutputPath = outputPath
	return result
}

// GetLayerPath returns the expected output path for a layer
func GetLayerPath(outputDir string, coords tile.Coords, layer geojson.LayerType) string {
	return filepath.Join(outputDir, fmt.Sprintf("%s_%s.png", coords.String(), layer))
}

// LayerExists checks if a layer file has already been rendered
func LayerExists(outputDir string, coords tile.Coords, layer geojson.LayerType) bool {
	path := GetLayerPath(outputDir, coords, layer)
	_, err := os.Stat(path)
	return err == nil
}
