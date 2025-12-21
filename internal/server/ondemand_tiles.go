package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MeKo-Tech/watercolormap/internal/pipeline"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
)

type OnDemandTilesConfig struct {
	TilesDir                 string
	StylesDir                string
	TexturesDir              string
	PNGCompression           string
	CacheControl             string
	BaseTileSize             int
	Seed                     int64
	MaxConcurrentGenerations int
	GenerationTimeout        time.Duration
	KeepLayers               bool
	GenerateMissing          bool
	DisableCache             bool
}

type OnDemandTiles struct {
	ds     pipeline.DataSource
	logger *slog.Logger
	sem    chan struct{}
	locks  sync.Map
	gens   sync.Map
	cfg    OnDemandTilesConfig
}

func NewOnDemandTiles(ds pipeline.DataSource, cfg OnDemandTilesConfig, logger *slog.Logger) (*OnDemandTiles, error) {
	if cfg.TilesDir == "" {
		cfg.TilesDir = "./tiles"
	}
	if cfg.StylesDir == "" {
		cfg.StylesDir = filepath.Join("assets", "styles")
	}
	if cfg.TexturesDir == "" {
		cfg.TexturesDir = filepath.Join("assets", "textures")
	}
	if cfg.BaseTileSize <= 0 {
		cfg.BaseTileSize = 256
	}
	if cfg.MaxConcurrentGenerations <= 0 {
		cfg.MaxConcurrentGenerations = 1
	}
	if cfg.GenerationTimeout <= 0 {
		cfg.GenerationTimeout = 2 * time.Minute
	}
	if cfg.CacheControl == "" {
		cfg.CacheControl = "no-store"
	}

	t := &OnDemandTiles{ds: ds, cfg: cfg, logger: logger, sem: make(chan struct{}, cfg.MaxConcurrentGenerations)}
	return t, nil
}

func (t *OnDemandTiles) Handler() http.Handler {
	return http.HandlerFunc(t.serveTile)
}

func (t *OnDemandTiles) serveTile(w http.ResponseWriter, r *http.Request) {
	// Allow browser-based playgrounds (including GitHub Pages) to request tiles.
	// Note: HTTPS pages cannot fetch from HTTP backends due to mixed-content rules.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	coords, suffix, ok := parseTilePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	filename := coords.String() + suffix + ".png"
	fullPath := filepath.Join(t.cfg.TilesDir, filename)

	w.Header().Set("Cache-Control", t.cfg.CacheControl)

	if !t.cfg.DisableCache {
		if fileExists(fullPath) {
			http.ServeFile(w, r, fullPath)
			return
		}
	}

	if !t.cfg.GenerateMissing {
		http.Error(w, fmt.Sprintf("tile not found: %s", filename), http.StatusNotFound)
		return
	}

	lockKey := filename
	mu := t.getLock(lockKey)
	mu.Lock()
	defer mu.Unlock()

	if !t.cfg.DisableCache {
		if fileExists(fullPath) {
			http.ServeFile(w, r, fullPath)
			return
		}
	}

	select {
	case t.sem <- struct{}{}:
		defer func() { <-t.sem }()
	case <-r.Context().Done():
		http.Error(w, "request cancelled", http.StatusRequestTimeout)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), t.cfg.GenerationTimeout)
	defer cancel()

	force := t.cfg.DisableCache
	gen, err := t.getGenerator(tileSizeForSuffix(t.cfg.BaseTileSize, suffix))
	if err != nil {
		t.log().Error("failed to init generator", "error", err)
		http.Error(w, "failed to init generator", http.StatusInternalServerError)
		return
	}

	start := time.Now()
	_, _, err = gen.Generate(ctx, coords, force, suffix, nil)
	if err != nil {
		t.log().Error("failed to generate tile", "coords", coords.String(), "suffix", suffix, "error", err)
		http.Error(w, fmt.Sprintf("failed to generate tile %s: %v", coords.String()+suffix, err), http.StatusBadGateway)
		return
	}
	t.log().Info("tile generated on-demand", "coords", coords.String(), "suffix", suffix, "ms", time.Since(start).Milliseconds())

	if !fileExists(fullPath) {
		http.Error(w, "tile generation completed but file missing on disk", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func (t *OnDemandTiles) getGenerator(tileSize int) (*pipeline.Generator, error) {
	if v, ok := t.gens.Load(tileSize); ok {
		return v.(*pipeline.Generator), nil
	}

	g, err := pipeline.NewGenerator(
		t.ds,
		t.cfg.StylesDir,
		t.cfg.TexturesDir,
		t.cfg.TilesDir,
		tileSize,
		t.cfg.Seed,
		t.cfg.KeepLayers,
		t.logger,
		pipeline.GeneratorOptions{PNGCompression: t.cfg.PNGCompression},
	)
	if err != nil {
		return nil, err
	}

	actual, _ := t.gens.LoadOrStore(tileSize, g)
	return actual.(*pipeline.Generator), nil
}

func (t *OnDemandTiles) getLock(key string) *sync.Mutex {
	if v, ok := t.locks.Load(key); ok {
		return v.(*sync.Mutex)
	}
	mu := &sync.Mutex{}
	actual, _ := t.locks.LoadOrStore(key, mu)
	return actual.(*sync.Mutex)
}

func (t *OnDemandTiles) log() *slog.Logger {
	if t.logger != nil {
		return t.logger
	}
	return slog.Default()
}

func parseTilePath(requestPath string) (tile.Coords, string, bool) {
	// Expect: /tiles/z13_x4317_y2692.png or /tiles/z13_x4317_y2692@2x.png
	if !strings.HasPrefix(requestPath, "/tiles/") {
		return tile.Coords{}, "", false
	}
	base := path.Base(requestPath)
	if !strings.HasSuffix(base, ".png") {
		return tile.Coords{}, "", false
	}
	name := strings.TrimSuffix(base, ".png")
	suffix := ""
	if strings.HasSuffix(name, "@2x") {
		suffix = "@2x"
		name = strings.TrimSuffix(name, "@2x")
	}

	coords, err := tile.ParseCoords(name)
	if err != nil {
		return tile.Coords{}, "", false
	}
	return coords, suffix, true
}

func tileSizeForSuffix(base int, suffix string) int {
	if suffix == "@2x" {
		return base * 2
	}
	return base
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !st.IsDir()
}
