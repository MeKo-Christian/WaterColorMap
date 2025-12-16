# Quick Start: WASM Playground

## 🚀 Run Locally

```bash
# Build and serve on http://localhost:8000/wasm-playground/
just build-wasm-local
```

Then open your browser to: **http://localhost:8000/wasm-playground/**

## 📦 GitHub Pages Deployment

The playground will be automatically deployed to GitHub Pages when you push to the main branch.

Access it at: **`https://[your-username].github.io/[repo-name]/wasm-playground/`**

(The exact URL depends on your repository settings)

## 🗺️ Features

- **Interactive Map**: Zoom and pan using Leaflet
- **On-Demand Tiles**: Tiles are generated when needed
- **Client-Side Caching**: Uses IndexedDB to store tiles locally
- **Cache Controls**: Clear cache or toggle rendering mode via buttons
- **Status Display**: Real-time info on cache hits, tile loading

## ⚙️ How It Works

1. **Map Navigation** → Tile coordinates are extracted
2. **Cache Check** → IndexedDB is queried first
3. **Cache Hit** → Tile loads instantly
4. **Cache Miss** → WASM module delegates to backend server
5. **Tile Display** → Map updates with new tile

## 🔧 Backend Integration

For full functionality, run the server in another terminal:

```bash
# Terminal 1: Start the playground
just build-wasm-local

# Terminal 2: Run the backend server
./bin/watercolormap serve --addr 127.0.0.1:8080
```

The playground will then be able to request tiles from `http://localhost:8080/tiles/`.

## 📋 Build Artifacts

After running `just build-wasm`, you'll have:

- `docs/wasm-playground/wasm.wasm` (3.1 MB) - Compiled Go module
- `docs/wasm-playground/wasm_exec.js` (17 KB) - Go's WASM runtime
- `docs/wasm-playground/index.html` - Map UI
- `docs/wasm-playground/wasm.js` - JavaScript controller

## 🐛 Troubleshooting

**"WASM module failed to load"**
- Check that `wasm.wasm` and `wasm_exec.js` are in the same directory
- Check browser console for specific errors

**"Tiles not generating"**
- Verify backend server is running on port 8080
- Check browser Network tab for failed requests

**"Cache seems stale"**
- Click "Clear Cache" button in the info box
- Or use browser DevTools: Application → Storage → IndexedDB

## 📚 Documentation

See [WASM-PLAYGROUND-IMPLEMENTATION.md](./WASM-PLAYGROUND-IMPLEMENTATION.md) for:
- Complete architecture overview
- File structure
- Performance characteristics
- Integration points
- Future enhancements

See [docs/wasm-playground/README.md](./docs/wasm-playground/README.md) for:
- Build instructions
- Local testing guide
- GitHub Pages setup
