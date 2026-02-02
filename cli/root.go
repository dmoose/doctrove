package cli

import (
	"fmt"
	"os"

	"github.com/dmoose/llmshadow/internal/engine"
	"github.com/spf13/cobra"
)

var rootDir string

var rootCmd = &cobra.Command{
	Use:   "llmshadow",
	Short: "Mirror and track websites' LLM-targeted content",
	Long:  "A tool that discovers, downloads, and maintains local mirrors of websites' LLM-targeted content with change tracking.",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootDir, "dir", "d", ".", "root directory for the llmshadow workspace")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newEngine creates an Engine from the current flags.
func newEngine() (*engine.Engine, error) {
	e, err := engine.New(rootDir)
	if err != nil {
		return nil, fmt.Errorf("initializing: %w", err)
	}
	return e, nil
}
