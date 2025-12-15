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

## Phase 2: Rendering Base Map Layers

### 2.1 Layer Design and Planning
- [x] Define layer hierarchy (land, water, parks, roads, civic areas)
- [x] Determine rendering order for compositing
- [x] Plan color coding scheme for mask extraction
- [x] Document layer specifications and purposes
- [x] Create test checklist for visual verification

**Status**: ✅ **COMPLETE**

**Documentation**: [docs/2.1-layer-design.md](docs/2.1-layer-design.md)

**Layer Color Mapping**:
- Water: #0000FF (blue) → water.png texture
- Land: #C4A574 (tan) → land.png texture
- Parks: #00FF00 (green) → green.png texture
- Civic: #C080C0 (lilac) → lilac.png texture
- Roads: #FFFF00 (yellow) → yellow.png texture
- Gray: (reserved) → gray.png texture

### 2.2 Mapnik Style Configuration
- [x] Create CartoCSS/XML style for land areas (solid color)
- [x] Create style for water bodies (distinct color)
- [x] Create style for parks/green spaces (distinct color)
- [x] Create style for major roads (distinct color)
- [x] Create style for civic/building areas (optional)
- [x] Test styles render correct features from data source

**Status**: ✅ **COMPLETE**

**Implementation**: 5 Mapnik XML styles in `assets/styles/layers/`:
- `water.xml` - Water bodies and waterways (blue)
- `land.xml` - Background fill (tan)
- `parks.xml` - Green spaces (green)
- `civic.xml` - Buildings (lilac)
- `roads.xml` - Roads with width scaling (yellow)

### 2.3 Tile System Implementation
- [x] Implement Web Mercator projection utilities
- [x] Create tile coordinate system (z/x/y) handler
- [x] Implement bounding box calculation for tiles
- [x] Create tile range utilities for batch generation
- [x] Test tile boundary alignment

**Status**: ✅ **COMPLETE**

**Implementation**: `internal/tile/coords.go`
- Tile coordinate system (Coords struct)
- WGS84 and Web Mercator bounds calculation
- Coordinate conversions
- TileRange for batch processing
- Full test coverage (all tests passing)

### 2.4 Multi-Pass Rendering Implementation
- [x] Implement multi-pass rendering (separate image per layer)
- [x] Create layer isolation logic
- [x] Implement GeoJSON conversion from OSM data
- [x] Create temporary file management for datasources
- [x] Integrate Mapnik XML style loading
- [x] Implement rendering for each layer type
- [x] Test rendering on sample tile (Phase 2.5)

**Status**: ✅ **COMPLETE**

**Implementation**:
- `internal/renderer/multipass.go` - Multi-pass rendering engine
- `internal/geojson/converter.go` - OSM to GeoJSON conversion
- Enhanced `internal/renderer/mapnik.go` with LoadXML and SetBounds methods

**Features**:
- Renders each layer separately: Land → Water → Parks → Civic → Roads
- Converts OSM features to GeoJSON per layer
- Replaces datasource placeholders in Mapnik styles
- Handles layers with no features (skips rendering)
- Manages temporary files automatically
- Outputs separate PNG for each layer

**Test Results**:
- ✅ Multi-pass renderer creation test passed
- ✅ Layer path helper functions tested
- ✅ GeoJSON conversion fully tested (7/7 tests passing)

### 2.5 Initial Testing
- [x] Select test tile covering central area (z13_x4317_y2692)
- [x] Render single test tile with all layers
- [x] Verify land layer fills entire background
- [ ] Verify water features align with rivers/lakes
- [ ] Verify parks appear in correct locations
- [x] Verify civic buildings render
- [ ] Verify roads render with correct widths
- [ ] Check tile edge alignment
- [ ] Verify color accuracy (no anti-aliasing issues)
- [ ] Document rendering issues and edge cases

**Status**: 🟡 **IN PROGRESS** - Basic rendering verified; visual alignment checks pending

## Phase 3: Image Processing - Watercolor Effect

### 3.1 Mask Processing Pipeline
- [x] Implement binary mask extraction from layer images
- [x] Implement Gaussian blur for mask softening
- [x] Research and implement Perlin noise generation in Go
- [x] Create tileable 1024x1024 Perlin noise texture
- [x] Implement noise overlay on blurred mask
- [x] Implement threshold function for mask sharpening
- [x] Add optional antialiasing for mask edges
- [x] Test pipeline on single feature layer

**Status**: ✅ **COMPLETE** - Full watercolor mask processing pipeline implemented and tested!

**Implementation**: `internal/mask/processor.go`
- Binary mask extraction from colored layer images
- Gaussian blur for edge softening (using disintegration/gift)
- Perlin noise generation (using aquilax/go-perlin)
- Noise overlay with configurable strength
- Threshold function for mask sharpening
- Antialiasing for smooth edges
- Full test coverage (7/7 tests passing)

**Test Results**:
- ✅ ExtractBinaryMask: Converts colored layers to binary masks
- ✅ GaussianBlur: Softens edges with configurable sigma
- ✅ GeneratePerlinNoise: Creates deterministic noise textures
- ✅ ApplyNoiseToMask: Overlays noise for organic edges
- ✅ ApplyThreshold: Sharpens masks after noise application
- ✅ AntialiasEdges: Final edge smoothing
- ✅ TestWatercolorPipeline: Complete integration test with 256x256 circular feature

**Pipeline Workflow**:
1. Extract binary mask from rendered layer (transparent → black, feature → white)
2. Apply Gaussian blur to soften edges
3. Generate Perlin noise texture
4. Overlay noise on blurred mask for organic variation
5. Apply threshold to sharpen the noisy mask
6. Apply light antialiasing for smooth final edges

This creates the characteristic watercolor "bleeding" effect with organic, hand-painted edges.

### 3.2 Noise Consistency Across Tiles
- [x] Implement deterministic noise positioning
- [x] Align noise texture to global tile grid
- [x] Test noise continuity at tile boundaries
- [x] Verify adjacent tiles have matching noise patterns
- [x] Document noise alignment strategy

**Status**: ✅ **COMPLETE** - Perlin noise sampling is aligned to a global grid to avoid seams.

**Implementation**: `internal/mask/processor.go`
- Added `GeneratePerlinNoiseWithOffset` for offset-aware sampling; legacy generator delegates to it.

**Tests**: `internal/mask/processor_test.go`
- Alignment tests for horizontal and vertical neighbors against a shared reference field.

**Documentation**: [docs/3.2-noise-consistency-across-tiles.md](docs/3.2-noise-consistency-across-tiles.md)

### 3.3 Texture Application
- [x] Implement texture tiling/scaling to tile size
- [x] Apply processed mask as alpha channel to texture
- [x] Implement color tinting for generic textures
- [x] Create texture variants for each layer type
- [x] Test texture application on sample masks
- [x] Verify seamless texture edges

**Status**: ✅ **COMPLETE** - Texture tiling, masking, tinting, and layer defaults implemented with tests.

**Implementation**: `internal/texture/processor.go`
- Global-offset tiling to keep seams invisible across tiles
- Mask-to-alpha application with automatic texture tiling
- Tinting utility preserving alpha
- Default layer-to-texture mapping (land, water, parks, civic, roads)

**Tests**: `internal/texture/processor_test.go`
- Seam alignment across horizontal/vertical neighbors
- Offset sampling correctness
- Mask application correctness
- Tint blending correctness

**Documentation**: [docs/3.3-texture-application.md](docs/3.3-texture-application.md)

### 3.4 Edge Darkening Effect
- [x] Implement edge detection on original mask
- [x] Create secondary blur for edge outline
- [x] Extract outer halo from blurred mask
- [x] Implement edge outline tapering
- [x] Apply darker color to edge overlay
- [x] Composite edge layer onto painted layer
- [x] Test edge effect on various feature types
- [x] Fine-tune edge thickness and darkness

**Status**: ✅ **COMPLETE** - Edge halo extraction, tapering, and darkening overlay implemented with tests.

**Implementation**: `internal/mask/edge.go`
- Edge halo via inner/outer blur differencing with normalization
- Optional gamma tapering for falloff control
- Dark overlay compositing that preserves base alpha

**Tests**: `internal/mask/edge_test.go`
- Halo presence on edges, near-zero center/outside
- Taper falloff correctness
- Darkening applied only where the edge mask indicates

**Documentation**: [docs/3.4-edge-darkening-effect.md](docs/3.4-edge-darkening-effect.md)

### 3.5 Layer-Specific Processing
- [x] Apply watercolor pipeline to land layer
- [x] Apply watercolor pipeline to water layer
- [x] Apply watercolor pipeline to parks layer
- [x] Apply watercolor pipeline to roads layer
- [x] Apply watercolor pipeline to civic areas (optional)
- [x] Verify each layer has appropriate visual style

**Status**: ✅ **COMPLETE** - Layer-aware watercolor processing with per-layer styles, textures, and edge darkening.

**Implementation**: `internal/watercolor/processor.go`
- Per-layer styles (texture, tint, edge color/strength, blur sigmas, gamma)
- Common pipeline params (tile size, blur, noise, threshold, antialias, seed, offsets)
- `DefaultParams` helper plus `PaintLayer` orchestrating mask → noise → texture → edge darkening

**Tests**: `internal/watercolor/processor_test.go`
- Validates masking/tinting/edge darkening and error on missing style

**Documentation**: [docs/3.5-layer-specific-processing.md](docs/3.5-layer-specific-processing.md)

### 3.6 Visual Quality Testing
- [ ] Test multiple blur radius values
- [ ] Test multiple threshold levels
- [ ] Compare results with original Stamen tiles
- [ ] Adjust parameters for optimal appearance
- [ ] Document final parameter values

## Phase 4: Compositing and Tile Delivery

### 4.1 Layer Compositing
- [ ] Implement layer compositing engine
- [ ] Define correct draw order (water, land, parks, civic, roads)
- [ ] Handle layer transparency correctly
- [ ] Implement pixel-perfect layer alignment
- [ ] Test compositing on single tile
- [ ] Verify layer overlap handling

### 4.2 Road Layer Special Handling
- [ ] Ensure road line widths scale with zoom level
- [ ] Apply appropriate blur for linear features
- [ ] Use reddish/orange tint for major roads
- [ ] Test road overlay on composite tile

### 4.3 Label Layer (Optional)
- [ ] Decide on label inclusion approach
- [ ] Render text labels via Mapnik (if included)
- [ ] Choose appropriate serif font
- [ ] Apply transparency to labels
- [ ] Test label legibility

### 4.4 Tile Edge Verification
- [ ] Generate multiple adjacent tiles
- [ ] Verify features align across boundaries
- [ ] Check noise pattern continuity
- [ ] Test at various zoom levels
- [ ] Fix any seam issues

### 4.5 Tile Output Format
- [ ] Implement PNG tile writer (256x256)
- [ ] Add support for Hi-DPI tiles (512x512)
- [ ] Optimize PNG compression
- [ ] Add tile metadata (attribution, etc.)
- [ ] Test tile loading in browser

### 4.6 Leaflet Integration
- [ ] Create basic HTML/Leaflet test page
- [ ] Set up local static file server (Go HTTP server)
- [ ] Configure Leaflet tile layer with local tiles
- [ ] Set appropriate zoom range (10-15)
- [ ] Add attribution text
- [ ] Test map interaction (pan, zoom)

### 4.7 Visual Tuning Iteration
- [ ] Review overall color saturation
- [ ] Adjust brightness if needed
- [ ] Test different textures for layers
- [ ] Fine-tune blur and threshold parameters
- [ ] Ensure park green distinguishes from land
- [ ] Verify road visibility
- [ ] Test at different zoom levels

### 4.8 Hanover Coverage Generation
- [ ] Define tile range for Hanover area
- [ ] Implement batch tile generation script
- [ ] Generate zoom level 10 tiles
- [ ] Generate zoom level 11 tiles
- [ ] Generate zoom level 12 tiles
- [ ] Generate zoom level 13 tiles
- [ ] Generate zoom level 14 tiles
- [ ] Generate zoom level 15 tiles
- [ ] Verify complete coverage in Leaflet

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
