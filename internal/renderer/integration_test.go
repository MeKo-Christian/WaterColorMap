package renderer

import (
	"os"
	"testing"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	if os.Getenv("WATERCOLORMAP_INTEGRATION") != "1" {
		t.Skip("skipping integration test (set WATERCOLORMAP_INTEGRATION=1 to enable)")
	}
}
