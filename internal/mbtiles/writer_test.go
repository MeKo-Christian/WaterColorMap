package mbtiles

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestWriter_New(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mbtiles")

	metadata := Metadata{
		Name:        "Test Tileset",
		Format:      "png",
		MinZoom:     10,
		MaxZoom:     14,
		Bounds:      [4]float64{9.5, 51.8, 9.9, 52.1},
		Center:      [3]float64{9.7, 51.95, 12},
		Attribution: "© Test",
		Description: "Test description",
		Type:        "baselayer",
		Version:     "1.0",
	}

	w, err := New(dbPath, metadata)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file was not created")
	}

	// Verify schema exists
	var count int
	err = w.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tiles'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected tiles table to exist, got count=%d", count)
	}

	// Verify metadata was inserted
	err = w.db.QueryRow("SELECT COUNT(*) FROM metadata").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query metadata: %v", err)
	}
	if count == 0 {
		t.Error("Expected metadata to be inserted")
	}
}

func TestWriter_WriteTile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mbtiles")

	metadata := Metadata{
		Name:   "Test",
		Format: "png",
	}

	w, err := New(dbPath, metadata)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Create fake PNG data
	pngData := []byte("fake png data")

	// Write a tile
	err = w.WriteTile(13, 4317, 2692, pngData)
	if err != nil {
		t.Fatalf("Failed to write tile: %v", err)
	}

	// Flush to ensure it's written
	err = w.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Verify tile was written
	var count int
	err = w.db.QueryRow("SELECT COUNT(*) FROM tiles").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tiles: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 tile, got %d", count)
	}

	// Verify TMS coordinate conversion
	var tileData []byte
	tmsY := (1 << 13) - 1 - 2692
	err = w.db.QueryRow("SELECT tile_data FROM tiles WHERE zoom_level=? AND tile_column=? AND tile_row=?",
		13, 4317, tmsY).Scan(&tileData)
	if err != nil {
		t.Fatalf("Failed to read tile: %v", err)
	}
	if len(tileData) == 0 {
		t.Error("Expected tile data to be stored")
	}
}

func TestWriter_BatchFlush(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mbtiles")

	metadata := Metadata{
		Name:   "Test",
		Format: "png",
	}

	w, err := New(dbPath, metadata)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write multiple tiles
	pngData := []byte("fake png data")
	for i := 0; i < 150; i++ {
		err = w.WriteTile(13, i, 100, pngData)
		if err != nil {
			t.Fatalf("Failed to write tile %d: %v", i, err)
		}
	}

	// Close should flush remaining tiles
	err = w.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Re-open and verify all tiles were written
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM tiles").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tiles: %v", err)
	}
	if count != 150 {
		t.Errorf("Expected 150 tiles, got %d", count)
	}
}

func TestWriter_ReplaceExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mbtiles")

	metadata := Metadata{
		Name:   "Test",
		Format: "png",
	}

	w, err := New(dbPath, metadata)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer w.Close()

	// Write a tile
	pngData1 := []byte("first version")
	err = w.WriteTile(13, 100, 200, pngData1)
	if err != nil {
		t.Fatalf("Failed to write first tile: %v", err)
	}
	w.Flush()

	// Write the same tile again with different data
	pngData2 := []byte("second version")
	err = w.WriteTile(13, 100, 200, pngData2)
	if err != nil {
		t.Fatalf("Failed to write second tile: %v", err)
	}
	w.Flush()

	// Verify only one tile exists (was replaced)
	var count int
	err = w.db.QueryRow("SELECT COUNT(*) FROM tiles").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tiles: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 tile (replaced), got %d", count)
	}
}
