# WASM Playground Build

This directory contains the browser playground (Leaflet + IndexedDB cache) and the
build artifacts needed to run the Go WebAssembly module.

**Files:**

- `index.html` - Main playground page (served from GitHub Pages)
- `wasm.js` - Browser code (Leaflet tile layer + IndexedDB caching)
- `wasm_bootstrap.js` - Loads and starts the Go WASM module (`wasm.wasm`)
- `wasm_exec.js` - Go WASM runtime (copied from your Go installation)
- `wasm.wasm` - Compiled WebAssembly module (built from `cmd/wasm`)

**Build:**

```bash
just build-wasm
```

**Local testing:**

```bash
just build-wasm-local
# Open http://localhost:8000/wasm-playground/
```

**Backend requirement:** Mapnik is a native dependency, so rendering does not run in the browser.
To actually see tiles, run a backend tile server locally:

```bash
./bin/watercolormap serve --addr 127.0.0.1:8080
```

The playground fetches tiles from the backend and caches them in IndexedDB.
You can override the backend base URL by opening:

`/wasm-playground/?backend=http://127.0.0.1:8080`
