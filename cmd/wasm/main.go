package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

// GenerateTileRequest represents a tile generation request from JS
type GenerateTileRequest struct {
	Zoom   int  `json:"zoom"`
	X      int  `json:"x"`
	Y      int  `json:"y"`
	HiDPI  bool `json:"hidpi"`
	Base64 bool `json:"base64"`
}

// generateTile is called from JavaScript to generate a tile
// In the browser, we delegate to a backend server or use a simplified renderer
func generateTile(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]string{"error": "missing arguments"}
	}

	reqStr := args[0].String()
	var req GenerateTileRequest
	if err := json.Unmarshal([]byte(reqStr), &req); err != nil {
		return map[string]string{"error": fmt.Sprintf("failed to parse request: %v", err)}
	}

	// In WASM environment, we cannot use Mapnik (native C++ library)
	// Instead, we provide a bridge to call a backend server or use simplified rendering
	tileKey := fmt.Sprintf("z%d_x%d_y%d", req.Zoom, req.X, req.Y)
	if req.HiDPI {
		tileKey += "@2x"
	}

	return map[string]string{
		"info": fmt.Sprintf("tile request: %s (use server backend)", tileKey),
		"note": "WASM in-browser rendering not available. Connect to a watercolormap serve instance.",
	}
}

// initGame is called on page load to set up the WASM module
func initGame(this js.Value, args []js.Value) interface{} {
	fmt.Println("WaterColorMap WASM module initialized")
	return map[string]string{"status": "ready"}
}

func main() {
	c := make(chan struct{})

	js.Global().Set("watercolorGenerateTile", js.FuncOf(generateTile))
	js.Global().Set("watercolorInit", js.FuncOf(initGame))

	fmt.Println("WaterColorMap WASM module loaded")
	<-c
}
