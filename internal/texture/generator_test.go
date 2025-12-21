package texture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateDefaultTexturesOutput(t *testing.T) {
	outputDir := filepath.Join("..", "..", "testdata", "output", "textures")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	result, err := WriteDefaultTextures(outputDir, 512, 1337, 1.0, 1.0, true)
	if err != nil {
		t.Fatalf("failed to generate textures: %v", err)
	}
	if len(result.Written) == 0 {
		t.Fatal("expected at least one texture to be written")
	}

	for _, path := range result.Written {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing generated texture %s: %v", path, err)
		}
	}
}
