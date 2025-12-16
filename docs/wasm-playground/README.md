# WASM Playground Build

This directory contains the build artifacts for the WebAssembly playground.

**Files:**
- `index.html` - Main playground page (served from GitHub Pages)
- `wasm.js` - JavaScript glue code (UI, caching, tile management)
- `wasm.wasm.js` - Generated WASM loader (build artifact)
- `wasm.wasm` - Compiled WebAssembly module (build artifact)

**Build:**
```bash
just build-wasm
```

**Local testing:**
```bash
cd docs/wasm-playground
python3 -m http.server 8000
# Open http://localhost:8000/
```

**Note:** The WASM module attempts to generate tiles in-browser using the full tile pipeline (Mapnik rendering, masking, texturing). Performance is intentionally slow without a backend cache.
