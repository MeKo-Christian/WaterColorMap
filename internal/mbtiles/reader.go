package mbtiles

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Reader reads tiles from an MBTiles database.
type Reader struct {
	db   *sql.DB
	path string
}

// OpenReader opens an MBTiles database for reading.
func OpenReader(path string) (*Reader, error) {
	// Open in read-only mode with immutable flag
	db, err := sql.Open("sqlite", path+"?mode=ro&immutable=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify schema exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tiles'").Scan(&count)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to verify schema: %w", err)
	}
	if count == 0 {
		db.Close()
		return nil, fmt.Errorf("database does not contain tiles table")
	}

	return &Reader{
		db:   db,
		path: path,
	}, nil
}

// ReadTile reads a tile from the database and returns ungzipped PNG data.
// Coordinates are in XYZ format and will be converted to TMS internally.
func (r *Reader) ReadTile(z, x, y int) ([]byte, error) {
	// Convert XYZ to TMS coordinates
	tmsY := (1 << z) - 1 - y

	var compressedData []byte
	err := r.db.QueryRow(
		"SELECT tile_data FROM tiles WHERE zoom_level=? AND tile_column=? AND tile_row=?",
		z, x, tmsY,
	).Scan(&compressedData)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tile not found: %d/%d/%d", z, x, y)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query tile: %w", err)
	}

	// Decompress gzip data
	uncompressed, err := gzipDecompress(compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress tile: %w", err)
	}

	return uncompressed, nil
}

// Metadata reads metadata from the database.
func (r *Reader) Metadata() (Metadata, error) {
	rows, err := r.db.Query("SELECT name, value FROM metadata")
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	meta := Metadata{}
	metaMap := make(map[string]string)

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return Metadata{}, fmt.Errorf("failed to scan metadata row: %w", err)
		}
		metaMap[name] = value
	}

	if err := rows.Err(); err != nil {
		return Metadata{}, fmt.Errorf("error iterating metadata: %w", err)
	}

	// Parse metadata fields
	meta.Name = metaMap["name"]
	meta.Format = metaMap["format"]
	meta.Attribution = metaMap["attribution"]
	meta.Description = metaMap["description"]
	meta.Type = metaMap["type"]
	meta.Version = metaMap["version"]

	if v, ok := metaMap["minzoom"]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			meta.MinZoom = i
		}
	}
	if v, ok := metaMap["maxzoom"]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			meta.MaxZoom = i
		}
	}

	// Parse bounds: "minLon,minLat,maxLon,maxLat"
	if v, ok := metaMap["bounds"]; ok {
		parts := strings.Split(v, ",")
		if len(parts) == 4 {
			for i, part := range parts {
				if f, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil {
					meta.Bounds[i] = f
				}
			}
		}
	}

	// Parse center: "lon,lat,zoom"
	if v, ok := metaMap["center"]; ok {
		parts := strings.Split(v, ",")
		if len(parts) == 3 {
			for i, part := range parts {
				if f, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil {
					meta.Center[i] = f
				}
			}
		}
	}

	return meta, nil
}

// Close closes the database connection.
func (r *Reader) Close() error {
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

// gzipDecompress decompresses gzip data.
func gzipDecompress(data []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	uncompressed, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}

	return uncompressed, nil
}
