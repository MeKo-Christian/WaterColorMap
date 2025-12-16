//go:build !js || !wasm
// +build !js !wasm

package main

import "fmt"

func main() {
	fmt.Println("cmd/wasm is intended to be built with GOOS=js GOARCH=wasm")
}
