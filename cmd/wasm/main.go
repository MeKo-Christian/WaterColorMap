//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

const defaultConcurrency = 4

// GenerateTileRequest represents a tile generation request from JS
type GenerateTileRequest struct {
	Zoom  int  `json:"zoom"`
	X     int  `json:"x"`
	Y     int  `json:"y"`
	HiDPI bool `json:"hidpi"`
}

type GenerateTileResponse struct {
	Key      string `json:"key"`
	Filename string `json:"filename"`
}

// getConcurrency returns the recommended number of concurrent operations.
// Uses navigator.hardwareConcurrency if available, otherwise defaults to 4.
func getConcurrency(_ js.Value, _ []js.Value) interface{} {
	navigator := js.Global().Get("navigator")
	if navigator.IsUndefined() || navigator.IsNull() {
		return defaultConcurrency
	}

	hwConcurrency := navigator.Get("hardwareConcurrency")
	if hwConcurrency.IsUndefined() || hwConcurrency.IsNull() {
		return defaultConcurrency
	}

	cores := hwConcurrency.Int()
	if cores < 1 {
		return defaultConcurrency
	}
	return cores
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

	// We cannot render Mapnik in WASM, but we *can* provide a canonical filename builder
	// so the browser code can reliably hit a backend `watercolormap serve` instance.
	suffix := ""
	if req.HiDPI {
		suffix = "@2x"
	}

	key := fmt.Sprintf("z%d_x%d_y%d%s", req.Zoom, req.X, req.Y, suffix)
	// syscall/js.ValueOf cannot convert arbitrary Go structs.
	// Return a JS-convertible object instead.
	return map[string]string{
		"key":      key,
		"filename": key + ".png",
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
	js.Global().Set("watercolorGetConcurrency", js.FuncOf(getConcurrency))
	js.Global().Set("watercolorInit", js.FuncOf(initGame))

	fmt.Println("WaterColorMap WASM module loaded")
	<-c
}
