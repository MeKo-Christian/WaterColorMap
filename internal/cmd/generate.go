package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/MeKo-Tech/watercolormap/internal/datasource"
	"github.com/MeKo-Tech/watercolormap/internal/pipeline"
	"github.com/MeKo-Tech/watercolormap/internal/tile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate map tiles",
	Long:  `Generate watercolor-styled map tiles for specified coordinates and zoom levels.`,
	RunE:  runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().IntP("zoom", "z", 13, "Zoom level")
	generateCmd.Flags().IntP("x", "x", 0, "X tile coordinate")
	generateCmd.Flags().IntP("y", "y", 0, "Y tile coordinate")
	generateCmd.Flags().Bool("force", false, "Force regeneration even if tile exists")

	if err := viper.BindPFlag("generate.zoom", generateCmd.Flags().Lookup("zoom")); err != nil {
		panic(fmt.Sprintf("failed to bind flag: %v", err))
	}
	if err := viper.BindPFlag("generate.x", generateCmd.Flags().Lookup("x")); err != nil {
		panic(fmt.Sprintf("failed to bind flag: %v", err))
	}
	if err := viper.BindPFlag("generate.y", generateCmd.Flags().Lookup("y")); err != nil {
		panic(fmt.Sprintf("failed to bind flag: %v", err))
	}
	if err := viper.BindPFlag("generate.force", generateCmd.Flags().Lookup("force")); err != nil {
		panic(fmt.Sprintf("failed to bind flag: %v", err))
	}
}

func runGenerate(cmd *cobra.Command, args []string) error {
	zoom := viper.GetInt("generate.zoom")
	x := viper.GetInt("generate.x")
	y := viper.GetInt("generate.y")
	force := viper.GetBool("generate.force")
	outputDir := viper.GetString("output-dir")
	dataSourceName := viper.GetString("data-source")

	if logger == nil {
		initLogging()
	}

	coords := tile.NewCoords(uint32(zoom), uint32(x), uint32(y))

	logger.Info("Starting tile generation",
		"coords", coords.String(),
		"output_dir", outputDir,
		"force", force,
		"data_source", dataSourceName,
	)

	if zoom < 0 || x < 0 || y < 0 {
		return fmt.Errorf("invalid coordinates: zoom/x/y must be non-negative")
	}

	var ds pipeline.DataSource
	switch dataSourceName {
	case "overpass":
		ds = datasource.NewOverpassDataSource("")
	default:
		return fmt.Errorf("unsupported data source: %s", dataSourceName)
	}

	stylesDir := filepath.Join("assets", "styles")
	texturesDir := filepath.Join("assets", "textures")

	const tileSize = 256
	const seed int64 = 1337

	gen, err := pipeline.NewGenerator(ds, stylesDir, texturesDir, outputDir, tileSize, seed, logger)
	if err != nil {
		return fmt.Errorf("failed to init generator: %w", err)
	}

	path, err := gen.Generate(context.Background(), coords, force)
	if err != nil {
		return fmt.Errorf("failed to generate tile: %w", err)
	}

	logger.Info("Tile generated", "coords", coords.String(), "path", path)
	return nil
}
