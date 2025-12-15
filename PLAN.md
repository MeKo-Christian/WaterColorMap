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

## Phase 3: Image Processing - Watercolor Effect ✅ COMPLETE

**Overview**: Implemented complete watercolor processing pipeline that transforms colored layer masks into textured, organic-edged features with characteristic "bleeding" effect.

**Pipeline Workflow**:
1. Extract binary mask from rendered layer (transparent → black, feature → white)
2. Apply Gaussian blur to soften edges
3. Overlay Perlin noise for organic variation (globally aligned to prevent tile seams)
4. Apply threshold to sharpen the noisy mask
5. Apply antialiasing for smooth final edges
6. Tile and apply texture with mask as alpha channel
7. Add edge darkening halo for depth

**Key Implementations**:
- `internal/mask/processor.go` - Binary mask extraction, Gaussian blur, Perlin noise generation with global offset alignment, threshold, antialiasing
- `internal/mask/edge.go` - Edge halo extraction via blur differencing, gamma tapering, dark overlay compositing
- `internal/texture/processor.go` - Global-offset texture tiling, mask-to-alpha application, color tinting
- `internal/watercolor/processor.go` - Complete pipeline orchestration with per-layer styles (texture, tint, edge color/strength, blur parameters)

**Critical Features**:
- **Noise Consistency**: Perlin noise aligned to global grid ensures seamless patterns across tile boundaries
- **Texture Tiling**: Global-offset sampling keeps texture seams invisible across tiles
- **Layer Styles**: Each layer has customized parameters (blur sigma, noise strength, edge darkness, color tint)
- **Edge Darkening**: Creates depth and watercolor characteristic darker outlines

**Test Coverage**: Full unit test suite for all pipeline components + parameter sensitivity tests

## Phase 4: Compositing and Tile Delivery

### 4.1 Layer Compositing
- [x] Implement layer compositing engine
- [x] Define correct draw order (water, land, parks, civic, roads)
- [x] Handle layer transparency correctly
- [x] Implement pixel-perfect layer alignment
- [x] Test compositing on single tile
- [x] Verify layer overlap handling

### 4.2 Road Layer Fidelity (per Stamen)
- [ ] Make road stroke widths zoom-aware in Mapnik (scale_denominator or per-zoom multiplier) so visual thickness stays consistent on 256/512 px tiles
- [ ] Keep road watercolor treatment readable: thinner blur/edge params for linear features, reddish/orange tint that survives compositing
- [ ] Add regression test comparing rendered road width/alpha at two zooms to prove scaling works

### 4.3 Labels Policy (Stamen default: none)
- [ ] Decide default posture: ship label-free tiles; document why (matches Stamen aesthetic)
- [ ] Optional path: Mapnik text style or external transparent label tiles; gate behind config flag
- [ ] Quick sanity render to confirm label legibility and halo/alpha settings if enabled

### 4.4 Seam & Alignment Verification
- [ ] Integration test rendering at least 2×2 adjacent tiles and diffing touching edges for pixel match
- [ ] Verify noise/texture offsets remain globally aligned; emit diagnostics when mismatches occur
- [ ] Document a manual Leaflet checklist for seam inspection

### 4.5 Output Formats & Hi-DPI
- [ ] Add `--hidpi`/config toggle to emit 512 px `@2x` tiles alongside 256 px output using existing tile-size plumbing
- [ ] Use `png.Encoder` with configurable compression level; benchmark default vs best speed
- [ ] Ensure filename pattern matches Leaflet retina expectations; cover with a small encoding test

### 4.6 Leaflet Demo & Local Serving
- [ ] Provide `just serve` (or tiny Go static server) to host `tiles/`
- [ ] Create minimal `docs/leaflet-demo.html` pointing at local tiles with attribution and sane zoom bounds
- [ ] Smoke-test demo after generating a Hanover sample set

### 4.7 Visual Tuning Controls
- [ ] Expose per-layer watercolor params (tint, blur sigma, noise strength, edge colors) via config with Phase 3 defaults
- [ ] Add golden/snapshot render for a known tile to catch regressions when tuning
- [ ] Document tuning guidance referencing the Stamen process steps (blur → noise → threshold → edge darkening)

### 4.8 Hanover Coverage Generation
- [ ] Add CLI flags for bbox/zoom-range batch generation (reuse `tile.TileRange`)
- [ ] Script batch generation for Hanover (z10–15) with progress logging, `--force`, and resumable output dirs
- [ ] Verify the produced set in the Leaflet demo and record bounds/zooms used

## Phase 5: Scaling and Modern Improvements

### 5.1 Data Scaling Strategy
- [ ] Plan regional database approach
- [ ] Evaluate vector tile input option
- [ ] Document data management for large regions
- [ ] Plan storage requirements
- [ ] Design data update pipeline

### 5.2 Parallel Tile Rendering
- [ ] Implement worker pool for tile generation
- [ ] Add goroutine-based parallel processing
- [ ] Implement database connection pooling
- [ ] Add progress tracking and logging
- [ ] Test parallel rendering performance
- [ ] Optimize worker count

### 5.3 Multi-Zoom Generation
- [ ] Define zoom range strategy (0-5: Natural Earth, 6-9: country, 10+: OSM)
- [ ] Implement zoom-specific data filtering
- [ ] Create generalized rendering for low zooms
- [ ] Test rendering at each zoom range
- [ ] Document zoom level characteristics

### 5.4 Tile Storage Format
- [ ] Research MBTiles format
- [ ] Implement MBTiles writer
- [ ] Convert folder tiles to MBTiles
- [ ] Test MBTiles serving
- [ ] Document MBTiles usage

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

## References

- [Stamen Watercolor Process](https://stamen.com/watercolor-process-3dd5135861fe/)
- [Stamen Watercolor Textures](https://stamen.com/watercolor-textures-15de97a4ad8b/)
- [Stamen Watercolor GitHub](https://github.com/stamen/watercolor)
- [OpenStreetMap Data](https://www.openstreetmap.org/)
- [Geofabrik Downloads](https://download.geofabrik.de/)
- [Natural Earth Data](https://www.naturalearthdata.com/)
