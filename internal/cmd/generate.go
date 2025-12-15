package cmd

import (
	"fmt"

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

	viper.BindPFlag("generate.zoom", generateCmd.Flags().Lookup("zoom"))
	viper.BindPFlag("generate.x", generateCmd.Flags().Lookup("x"))
	viper.BindPFlag("generate.y", generateCmd.Flags().Lookup("y"))
	viper.BindPFlag("generate.force", generateCmd.Flags().Lookup("force"))
}

func runGenerate(cmd *cobra.Command, args []string) error {
	zoom := viper.GetInt("generate.zoom")
	x := viper.GetInt("generate.x")
	y := viper.GetInt("generate.y")
	force := viper.GetBool("generate.force")
	outputDir := viper.GetString("output-dir")

	fmt.Printf("Generating tile: z%d/x%d/y%d\n", zoom, x, y)
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Printf("Force regeneration: %v\n", force)

	// TODO: Implement tile generation logic
	return fmt.Errorf("tile generation not yet implemented")
}
