package cmd

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/MeKo-Tech/watercolormap/internal/datasource"
	"github.com/MeKo-Tech/watercolormap/internal/pipeline"
	"github.com/MeKo-Tech/watercolormap/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve tiles and demo UI (optionally generating missing tiles on-demand)",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("addr", "127.0.0.1:8080", "Listen address (host:port)")
	serveCmd.Flags().String("tiles-dir", "", "Directory containing tiles (defaults to --output-dir)")
	serveCmd.Flags().String("demo-dir", filepath.Join("docs", "leaflet-demo"), "Directory for demo static files")

	serveCmd.Flags().Bool("generate-missing", true, "Generate missing tiles on-demand and cache them to disk")
	serveCmd.Flags().Bool("disable-cache", false, "Always regenerate tiles (still writes to disk)")
	serveCmd.Flags().Int("max-concurrent-generations", runtime.NumCPU(), "Max concurrent tile generations (default: number of CPUs)")
	serveCmd.Flags().Duration("generation-timeout", 2*time.Minute, "Timeout per tile generation")
	serveCmd.Flags().String("cache-control", "no-store", "Cache-Control header for served tiles")

	serveCmd.Flags().Int("tile-size", 256, "Base tile size in pixels (256; @2x requests render 512)")
	serveCmd.Flags().String("png-compression", "default", "PNG compression (default, speed, best, none)")
	serveCmd.Flags().Int64("seed", 1337, "Deterministic seed for noise/texture alignment")
	serveCmd.Flags().Bool("keep-layers", false, "Keep intermediate rendered layer PNGs for debugging")

	mustBind := func(key string, name string) {
		if err := viper.BindPFlag(key, serveCmd.Flags().Lookup(name)); err != nil {
			panic(fmt.Sprintf("failed to bind flag: %v", err))
		}
	}

	mustBind("serve.addr", "addr")
	mustBind("serve.tiles_dir", "tiles-dir")
	mustBind("serve.demo_dir", "demo-dir")
	mustBind("serve.generate_missing", "generate-missing")
	mustBind("serve.disable_cache", "disable-cache")
	mustBind("serve.max_concurrent_generations", "max-concurrent-generations")
	mustBind("serve.generation_timeout", "generation-timeout")
	mustBind("serve.cache_control", "cache-control")

	mustBind("serve.tile_size", "tile-size")
	mustBind("serve.png_compression", "png-compression")
	mustBind("serve.seed", "seed")
	mustBind("serve.keep_layers", "keep-layers")
}

func runServe(cmd *cobra.Command, args []string) error {
	if logger == nil {
		initLogging()
	}

	addr := viper.GetString("serve.addr")
	tilesDir := viper.GetString("serve.tiles_dir")
	if tilesDir == "" {
		tilesDir = viper.GetString("output-dir")
	}
	demoDir := viper.GetString("serve.demo_dir")
	generateMissing := viper.GetBool("serve.generate_missing")
	disableCache := viper.GetBool("serve.disable_cache")
	maxConc := viper.GetInt("serve.max_concurrent_generations")
	genTimeout := viper.GetDuration("serve.generation_timeout")
	cacheControl := viper.GetString("serve.cache_control")

	baseTileSize := viper.GetInt("serve.tile_size")
	pngCompression := viper.GetString("serve.png_compression")
	seed := viper.GetInt64("serve.seed")
	keepLayers := viper.GetBool("serve.keep_layers")

	dataSourceName := viper.GetString("data-source")
	var ds pipeline.DataSource
	switch dataSourceName {
	case "overpass":
		ds = datasource.NewOverpassDataSource("")
	default:
		return fmt.Errorf("unsupported data source: %s", dataSourceName)
	}

	od, err := server.NewOnDemandTiles(ds, server.OnDemandTilesConfig{
		TilesDir:                 tilesDir,
		StylesDir:                filepath.Join("assets", "styles"),
		TexturesDir:              filepath.Join("assets", "textures"),
		BaseTileSize:             baseTileSize,
		Seed:                     seed,
		KeepLayers:               keepLayers,
		PNGCompression:           pngCompression,
		GenerateMissing:          generateMissing,
		DisableCache:             disableCache,
		MaxConcurrentGenerations: maxConc,
		GenerationTimeout:        genTimeout,
		CacheControl:             cacheControl,
	}, logger)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/demo/", http.StatusFound)
	})

	// Demo UI
	fs := http.FileServer(http.Dir(demoDir))
	mux.Handle("/demo/", http.StripPrefix("/demo/", fs))

	// Tiles (with optional on-demand generation)
	mux.Handle("/tiles/", withCORS(od.Handler()))

	logger.Info("demo server listening",
		"addr", addr,
		"tiles_dir", tilesDir,
		"demo_dir", demoDir,
		"generate_missing", generateMissing,
		"max_concurrent_generations", maxConc,
	)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
