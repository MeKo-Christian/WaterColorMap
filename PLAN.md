# Watercolor Map Tiles - Implementation Plan

This document outlines the complete implementation plan for creating Stamen Watercolor-style map tiles in Go, starting with Hanover and eventually scaling globally.

## Phase 1: Data Preparation and Tool Setup ✅ COMPLETE

### 1.1-1.2 Data & Tile Infrastructure

- [x] Tile coordinate system (z/x/y) design and implementation
- [x] Flat tile storage structure (`tiles/z{zoom}_x{x}_y{y}.png`)
- [x] OSM data fetching via Overpass API (`internal/datasource/overpass.go`)
- [x] Bounding box and tile range utilities (`internal/tile/coords.go`)

**Tested**: z13_x4317_y2692 → 2,531 features (86 water, 87 parks, 621 roads, 1,736 buildings, 1 civic) in 1.9s

### 1.3-1.4 Rendering Stack

- [x] **Mapnik 3.1.0** (omniscale/go-mapnik v2.0.1) for map rendering
- [x] Web Mercator projection (EPSG:3857), 256×256 PNG output
- [x] Supporting libraries: paulmach/orb, fogleman/gg, disintegration/gift
- [x] CartoCSS/XML style support with Docker setup (Dockerfile, Justfile)

**Workflow**: Mapnik renders base layers → mask extraction → watercolor effects → composite tiles

### 1.5 Textures

- [x] 6 seamless 1024×1024 PNG textures (land, water, green, gray, lilac, yellow) ready in `assets/textures/`

### 1.6-1.7 Project Setup

- [x] Go structure (cmd/, internal/, pkg/, assets/), go.mod initialized
- [x] Configuration system with YAML support
- [x] Development environment fully prepared

## Phase 2: Rendering Base Map Layers ✅ COMPLETE

**Overview**: Implemented multi-pass Mapnik rendering system that generates separate PNG masks for each map layer (land, water, parks, civic, roads). Each layer uses distinct colors for downstream mask extraction and texture application.

**Layer Color Mapping**:

- Water: #0000FF (blue) → water.png texture
- Land: #C4A574 (tan) → land.png texture
- Parks: #00FF00 (green) → green.png texture
- Civic: #C080C0 (lilac) → lilac.png texture
- Roads: #FFFF00 (yellow) → yellow.png texture

**Key Implementations**:

- `internal/renderer/multipass.go` - Multi-pass rendering engine with 128px Mapnik buffer for seamless tile edges
- `internal/renderer/mapnik.go` - Mapnik wrapper with map object reset for layer isolation
- `internal/geojson/converter.go` - OSM to GeoJSON conversion
- `internal/tile/coords.go` - Web Mercator projection and tile coordinate system
- `assets/styles/layers/` - Mapnik XML styles for each layer

**Critical Fixes**:

- **Layer Isolation**: Mapnik map object reset prevents layer contamination in multi-pass rendering
- **Edge Alignment**: 128-pixel buffer (50% of tile size) ensures features render seamlessly across tile boundaries
- **Anti-aliasing**: Tests handle premultiplied alpha and perspective-dependent color variations (tolerance: 60)

**Test Coverage**: 68 unit tests + integration tests rendering 3×3 tile grids with layer separation and edge alignment verification

## Phase 3: Image Processing - Watercolor Effect (Stamen-Aligned Revision) 🟨 IN PROGRESS

**Why this revision**: The current Phase 3 implementation largely processes each layer independently using its own alpha mask. The Stamen process relies on **cross-layer mask construction** (e.g., land is derived by inverting a combined “non-land” mask), and reuses progressively blurred masks for additional effects.

### 3.0 Current State (v1)

**What exists today** (works, but simplified):

- Per-layer mask pipeline: blur → noise → threshold → antialias
- Texture tiling/tinting using the mask as alpha
- Edge darkening halo (mask blur differencing)

**Where**:

- `internal/mask/processor.go`
- `internal/mask/edge.go`
- `internal/texture/processor.go`
- `internal/watercolor/processor.go`

**Main gap vs Stamen**:

- No explicit “water + roads” (sea + roads) union mask used as the foundation.
- No explicit **inversion step** to derive the land mask from that union.
- No explicit reuse of “even-more-blurred” masks as multiplicative/overlay shading layers per feature category.

### 3.1 Revised Core Mask Logic (alpha-only)

We treat all masks as **single-channel alpha masks** (grayscale 0–255), derived only from the rendered layer PNG alpha.

**Base masks** (from rendered layers):

- `waterMask` := alpha(layer=water)
- `roadsMask` := alpha(layer=roads)

**Combined non-land mask** (union):

- `nonLandMask` := max(waterMask, roadsMask)
  - (Optional later: include other “non-land” contributors if we decide they must punch holes, but start with water+roads as requested.)

**Fuzzy boundary mask** (Stamen step):

1. `blur1` := GaussianBlur(nonLandMask)
2. `noisy` := blur1 + PerlinNoise (applied to the same channel)
3. `hard` := Threshold(noisy) → hard black/white (transparent/opaque)
4. `aa` := Antialias(hard)

**Invert for land**:

- `landMask` := invert(aa)
  - This produces a land mask where “everything not water/roads” becomes the textured land region.

**Antialiasing strategy** (pick simplest first):

- Option A (simple): small blur kernel (`sigma ~ 0.3–0.8`) after threshold
- Option B (higher quality): supersample at 2× and downsample (only if needed)

### 3.2 Using the Mask for Texture + Shading

**Land texture application**:

1. Tile/tint the land texture (globally aligned)
2. Apply `landMask` as alpha

**Land darkening / pigment accumulation** (reuse the same foundation mask):

1. `landShadeMask` := GaussianBlur(landMask, larger sigma)
2. Use `landShadeMask` as a black/transparent overlay and multiply/overlay it onto the painted land.

This matches the “keep blurring and reuse as a darkening overlay” idea: it’s derived from the same mask field and stays consistent across tiles.

### 3.3 Apply Similar Logic to Other Layers

For other layers (parks/civic/water/roads), we keep the same _mask building blocks_ but ensure **correct masking relationships** before painting:

- `parksMask` := alpha(parks) AND landMask
- `civicMask` := alpha(civic) AND landMask
- `waterMask` := alpha(water)
- `roadsMask` := alpha(roads)

Then each layer gets:

1. blur → noise → threshold → antialias (applied to that layer’s mask)
2. texture application using the final mask as alpha
3. optional further-blur reuse as darkening overlay (layer-specific)

### 3.4 Work Items (to complete Phase 3 revision)

- [x] Add explicit mask composition ops (alpha extraction, union/max, intersect/min, invert) and unit tests.
- [x] Add a new “cross-layer mask construction” step before painting any layer.
- [x] Update the land pipeline to use `landMask := invert(process(nonLandMask))` instead of “land’s own alpha”.
- [x] Update parks/civic to be constrained to land (AND landMask).
- [x] Add a test that verifies land is fully excluded where water/roads are present.
- [x] Re-tune blur/noise/threshold parameters after behavior changes.

## Phase 4: Compositing and Tile Delivery

### 4.1 Layer Compositing

- [x] Implement layer compositing engine
- [x] Define correct draw order (water, land, parks, civic, roads)
- [x] Handle layer transparency correctly
- [x] Implement pixel-perfect layer alignment
- [x] Test compositing on single tile
- [x] Verify layer overlap handling

### 4.2 Road Layer Fidelity (per Stamen)

- [x] Make road stroke widths zoom-aware in Mapnik (scale_denominator or per-zoom multiplier) so visual thickness stays consistent on 256/512 px tiles
- [x] Keep road watercolor treatment readable: thinner blur/edge params for linear features, reddish/orange tint that survives compositing
- [x] Add regression test comparing rendered road width/alpha at two zooms to prove scaling works

### 4.3 Labels Policy (Stamen default: none)

- [x] Ship label-free tiles (matches Stamen aesthetic)
- [x] Keep Mapnik styles label-free (current state: no labels)

### 4.4 Seam & Alignment Verification

- [x] Use metatile padding + crop during generation to avoid blur/edge artifacts at tile borders
- [ ] Add an integration test rendering adjacent tiles and checking border deltas stay within tolerance
- [ ] Document a quick manual seam inspection checklist (Leaflet)

### 4.5 Output Formats & Hi-DPI

- [x] Add `--hidpi`/config toggle to emit 512px `@2x` tiles alongside 256px output
- [ ] Ensure watercolor offsets/noise/texture stay globally aligned between 256px and 512px outputs (same world anchoring)
- [x] Define the on-disk naming/layout for retina (`@2x`) and document the matching Leaflet config
- [x] Use `png.Encoder` with configurable compression level; keep defaults fast and add a reproducible “best compression” mode

### 4.6 Leaflet Demo & Local Serving

- [x] Add a dedicated demo server command (prefer `watercolormap serve`) for local viewing and sharing screenshots

- [x] Support serving tiles from the existing flat naming scheme (`tiles/z{z}_x{x}_y{y}.png` and `@2x`)
- [x] Provide a Leaflet demo page served by the same server (no external build tooling)

**Server requirements**

- [x] HTTP server with configurable listen address (default `127.0.0.1:8080`)
- [x] Configurable tile directory (default `./tiles`) and static assets root (default `./docs`)
- [x] Routes:
  - [x] `GET /healthz` → plain `ok`
  - [x] `GET /` → redirect to `/demo/`
  - [x] `GET /demo/` → serve the Leaflet demo page
  - [x] `GET /tiles/...` → serve tile PNGs from disk (with on-demand generation if missing)
- [x] Friendly 404 for missing tiles (include requested z/x/y in the response)
- [x] Correct headers for PNG (`Content-Type: image/png`) and optional dev-friendly caching (`Cache-Control: no-store` by default)
- [ ] Optional CORS toggle for tile requests (off by default; useful for embedding the demo elsewhere)

**Leaflet demo page requirements**

- [x] Minimal HTML (no build step) at `docs/leaflet-demo/index.html`
- [x] Uses Leaflet via CDN
- [x] Uses the demo server as the tile source (no hard-coded host; derive from `window.location`)
- [x] Tile URL strategy:
  - [x] Default: request tiles using the project's flat file naming scheme
  - [x] HiDPI: support `@2x` tiles via Leaflet `detectRetina` (or a simple DPR switch) when available
- [x] Sane defaults: initial view centered on Hanover, min/max zoom aligned with what we generate (Phase 4.8)
- [x] Attribution included on the map (OSM) and a short note that the style is "Watercolor-inspired"

**Developer ergonomics**

- [x] Add `just serve` to run the server against `./tiles` (and optionally `just demo` as an alias)
- [ ] Document quickstart in README: generate a tile set → run server → open browser URL

**Smoke test / acceptance**

- [ ] Generate a small Hanover set (e.g., a 3×3 grid at z13) and verify:
  - [ ] Demo page loads without console errors
  - [ ] Tiles load and pan smoothly
  - [ ] HiDPI tiles render when present
  - [ ] Missing tiles are generated on-demand and displayed
  - [ ] Regenerated tiles are cached to disk for subsequent requests

### 4.7 Visual Tuning Controls

- [ ] Expose per-layer watercolor params (tint, blur sigma, noise strength, edge colors) via config with Phase 3 defaults
- [ ] Add golden/snapshot render for a known tile to catch regressions when tuning
- [ ] Document tuning guidance referencing the Stamen process steps (blur → noise → threshold → edge darkening)

### 4.8 Hanover Coverage Generation

- [ ] Add CLI flags for bbox/zoom-range batch generation (reuse `tile.TileRange`)
- [ ] Script batch generation for Hanover (z10–15) with progress logging, `--force`, and resumable output dirs
- [ ] Verify the produced set in the Leaflet demo and record bounds/zooms used

### 4.9 TileJSON / Delivery Metadata

- [ ] Emit a minimal `tilejson.json` (bounds, min/max zoom, format, tile URL template) for the generated set
- [ ] Include required attribution text (Stamen-style / OSM) in the metadata and demo

## Phase 5: Scaling and Modern Improvements

### 5.1 Data Scaling Strategy

- [ ] Plan regional database approach
- [ ] Evaluate vector tile input option
- [ ] Document data management for large regions
- [ ] Plan storage requirements
- [ ] Design data update pipeline

### 5.2 Parallel Tile Rendering

- [x] Implement worker pool for tile generation (`internal/worker/pool.go`)
- [x] Add goroutine-based parallel processing (configurable worker count, defaults to NumCPU)
- [x] Implement database connection pooling (N/A - Overpass API, generators are per-worker)
- [x] Add progress tracking and logging (`internal/worker/progress.go`)
- [x] Test parallel rendering performance (unit tests in `internal/worker/pool_test.go`)
- [x] Optimize worker count (defaults to `runtime.NumCPU()`)
- [x] Add batch CLI command (`--bbox`, `--zoom-min`, `--zoom-max`, `--workers`, `--progress`)

### 5.3 Multi-Zoom Generation

- [ ] Define zoom range strategy (0-5: Natural Earth, 6-9: country, 10+: OSM)
- [ ] Implement zoom-specific data filtering
- [ ] Create generalized rendering for low zooms
- [ ] Test rendering at each zoom range
- [ ] Document zoom level characteristics

### 5.4 Tile Storage Format

- [x] Research MBTiles format
- [x] Implement MBTiles writer
- [x] Convert folder tiles to MBTiles
- [x] Test MBTiles serving
- [x] Document MBTiles usage

### 5.5 Tile Hosting Options

- [ ] Evaluate self-hosting requirements
- [ ] Research cloud storage options (S3, Azure Blob)
- [ ] Test CDN integration (CloudFront)
- [ ] Evaluate third-party providers (Mapbox, MapTiler)
- [ ] Document hosting recommendations
- [ ] Set up initial hosting solution

### 5.6 On-the-Fly Rendering Service

- [ ] Design Go tile server architecture
- [ ] Implement tile caching strategy
- [ ] Add cache hit/miss handling
- [ ] Implement LRU cache or Redis
- [ ] Test server under load
- [ ] Optimize for cache performance

### 5.6a Browser Playground (WebAssembly On-Demand)

- [x] Compile tile generator to WebAssembly (Go → WASM) using TinyGo or standard Go WASM toolchain
- [x] Create a minimal browser UI with Leaflet + IndexedDB/localStorage for client-side tile caching
- [x] Implement on-demand tile generation in the browser (fetch OSM data → render → cache → display)
  - Note: Actual rendering delegates to backend server since Mapnik can't run in browser; WASM provides canonical filename builder
- [x] Handle browser memory/performance constraints (limit concurrent generations, use web workers if needed)
- [x] Set up GitHub Actions CI workflow to build WASM artifact on commits
- [x] Deploy built WASM + demo HTML to GitHub Pages (gh-pages branch or Pages deployment)
- [x] Display rendering progress and cache status in the UI
- [x] Document browser limitations and expected slowness without proper caching backend
- [x] Add disclaimer that this is a proof-of-concept playground, not production-grade

**Note**: Rendering will be intentionally slow in the browser (seconds per tile) without backend caching. Useful for demonstration and exploration, but refer users to the hosted service for production use.

### 5.7 Data Update Pipeline

- [ ] Design periodic data refresh strategy
- [ ] Implement OSM diff application (optional)
- [ ] Create full re-render pipeline
- [ ] Schedule automated updates
- [ ] Test update process
- [ ] Document update procedures

### 5.8 Enhanced Textures

- [ ] Create zoom-specific textures
- [ ] Add coarse paper texture for low zoom
- [ ] Add fine detail textures for high zoom
- [ ] Implement texture selection by zoom
- [ ] Test visual consistency across zooms

### 5.9 Modern Enhancements

- [ ] Evaluate hillshading/relief integration
- [ ] Test DEM data overlay
- [ ] Implement subtle terrain shading
- [ ] Add paper grain effect (optional)
- [ ] Test overall aesthetic balance

### 5.10 Vector Data Integration

- [ ] Plan vector tile service for interactivity
- [ ] Set up parallel vector tile endpoint
- [ ] Test feature highlighting on hover
- [ ] Document vector integration approach

### 5.11 Performance Optimization

- [ ] Profile tile generation performance
- [ ] Optimize image processing bottlenecks
- [ ] Reduce memory usage
- [ ] Optimize database queries
- [ ] Test end-to-end generation speed

### 5.12 Documentation and Deployment

- [ ] Document complete installation process
- [ ] Create configuration guide
- [ ] Document troubleshooting steps
- [ ] Create user guide for custom textures
- [ ] Document API/tile serving interface
- [ ] Write deployment guide
- [ ] Create monitoring and maintenance guide

## Phase 6: Global Expansion

### 6.1 Data Preparation

- [ ] Download OSM planet file or regional extracts
- [ ] Import global data into PostGIS (or regional DBs)
- [ ] Verify data coverage and quality
- [ ] Document global data setup

### 6.2 Region Prioritization

- [ ] Identify high-priority regions for initial generation
- [ ] Plan generation schedule by region
- [ ] Allocate storage for global tiles
- [ ] Document regional coverage

### 6.3 Batch Generation

- [ ] Create global tile generation script
- [ ] Implement resume capability for interrupted runs
- [ ] Add error handling and retry logic
- [ ] Generate tiles by region/zoom
- [ ] Monitor generation progress
- [ ] Verify tile quality across regions

### 6.4 Quality Assurance

- [ ] Visual spot-checking of key cities
- [ ] Automated testing for tile validity
- [ ] Check tile edge alignment globally
- [ ] Verify color consistency
- [ ] Test at various zoom levels worldwide

### 6.5 Final Deployment

- [ ] Upload complete tile set to hosting
- [ ] Configure CDN for global delivery
- [ ] Set up monitoring and analytics
- [ ] Create public demo page
- [ ] Announce project completion

---

## Success Criteria

Each phase is considered complete when:

1. **Phase 1**: All tools installed, data imported, single test render succeeds
2. **Phase 2**: All layers render correctly for test tile, colors distinct
3. **Phase 3**: Watercolor effect applied, textures show properly, edges organic
4. **Phase 4**: Composite tiles seamless, Leaflet shows Hanover beautifully
5. **Phase 5**: Parallel rendering works, hosting deployed, updates automated
6. **Phase 6**: Global coverage achieved, performance acceptable, publicly accessible

## Notes

- Mark tasks complete only when fully verified
- Document issues and solutions as you encounter them
- Test incrementally - don't move ahead with broken foundations
- Keep the Stamen design philosophy in mind: artistic, organic, beautiful
- Maintain deterministic processing for seamless tile edges
- Balance authenticity with modern performance

## MBTiles Usage (Phase 5.4)

### Generate tiles directly to MBTiles

```bash
watercolormap generate --format=mbtiles \
  --output-file=hanover.mbtiles \
  --bbox=9.5,51.8,9.9,52.1 \
  --zoom-min=10 --zoom-max=15
```

For HiDPI tiles, two separate files are created:

- `hanover.mbtiles` (base 256px tiles)
- `hanover@2x.mbtiles` (512px tiles)

### Convert existing folder tiles to MBTiles

```bash
watercolormap convert \
  --input-dir=./tiles \
  --output=hanover.mbtiles \
  --name="WaterColorMap Hanover" \
  --bounds="9.5,51.8,9.9,52.1"
```

### Serve tiles from MBTiles

```bash
watercolormap serve --mbtiles=hanover.mbtiles --port=8080
```

MBTiles format provides:

- Single file portability (no thousands of individual files)
- Efficient storage with gzip compression
- Standard SQLite format compatible with most map tools
- TMS coordinate system (Y-axis inverted from XYZ)

## References

- [Stamen Watercolor Process](https://stamen.com/watercolor-process-3dd5135861fe/)
- [Stamen Watercolor Textures](https://stamen.com/watercolor-textures-15de97a4ad8b/)
- [Stamen Watercolor GitHub](https://github.com/stamen/watercolor)
- [OpenStreetMap Data](https://www.openstreetmap.org/)
- [Geofabrik Downloads](https://download.geofabrik.de/)
- [Natural Earth Data](https://www.naturalearthdata.com/)
- [MBTiles Specification](https://github.com/mapbox/mbtiles-spec)
