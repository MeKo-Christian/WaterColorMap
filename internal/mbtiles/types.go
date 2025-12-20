// Package mbtiles provides MBTiles format support for reading and writing tile databases.
package mbtiles

import "fmt"

// Metadata contains MBTiles metadata fields.
type Metadata struct {
	Name        string // Human-readable tileset identifier
	Format      string // Tile data type (png, jpg, webp, pbf)
	Attribution string // Attribution text
	Description string // Human-readable description
	Type        string // "baselayer" or "overlay"
	Version     string // Version string
	Bounds      [4]float64
	Center      [3]float64
	MinZoom     int // Minimum zoom level
	MaxZoom     int // Maximum zoom level
}

// ToMap converts Metadata to a map for database insertion.
func (m Metadata) ToMap() map[string]string {
	result := make(map[string]string)

	if m.Name != "" {
		result["name"] = m.Name
	}
	if m.Format != "" {
		result["format"] = m.Format
	}
	if m.MinZoom > 0 {
		result["minzoom"] = fmt.Sprintf("%d", m.MinZoom)
	}
	if m.MaxZoom > 0 {
		result["maxzoom"] = fmt.Sprintf("%d", m.MaxZoom)
	}
	if m.Bounds != [4]float64{} {
		result["bounds"] = fmt.Sprintf("%.6f,%.6f,%.6f,%.6f",
			m.Bounds[0], m.Bounds[1], m.Bounds[2], m.Bounds[3])
	}
	if m.Center != [3]float64{} {
		result["center"] = fmt.Sprintf("%.6f,%.6f,%d",
			m.Center[0], m.Center[1], int(m.Center[2]))
	}
	if m.Attribution != "" {
		result["attribution"] = m.Attribution
	}
	if m.Description != "" {
		result["description"] = m.Description
	}
	if m.Type != "" {
		result["type"] = m.Type
	}
	if m.Version != "" {
		result["version"] = m.Version
	}

	return result
}
