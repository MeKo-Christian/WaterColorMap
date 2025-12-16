#!/bin/bash
set -e

DEST_DIR="docs/wasm-playground"
GOROOT=$(go env GOROOT)

# Try multiple locations where wasm_exec.js might be found
SEARCH_PATHS=(
	"$GOROOT/misc/wasm/wasm_exec.js"
	"$GOROOT/lib/wasm/wasm_exec.js"
	"/usr/share/go-*/lib/wasm/wasm_exec.js"
	"/usr/local/go*/misc/wasm/wasm_exec.js"
	"/usr/local/go*/lib/wasm/wasm_exec.js"
)

FOUND_FILE=""
for pattern in "${SEARCH_PATHS[@]}"; do
	# Expand glob patterns and check first match
	for file in $pattern; do
		if [ -f "$file" ]; then
			FOUND_FILE="$file"
			break 2
		fi
	done
done

if [ -n "$FOUND_FILE" ]; then
	cp "$FOUND_FILE" "$DEST_DIR/wasm_exec.js"
	echo "Copied wasm_exec.js from: $FOUND_FILE"
else
	echo "Warning: wasm_exec.js not found in any standard location"
	exit 1
fi
