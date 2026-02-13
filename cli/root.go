package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dmoose/llmshadow/internal/engine"
	"github.com/spf13/cobra"
)

var (
	rootDir       string
	respectRobots bool
	jsonOutput    bool
)

var rootCmd = &cobra.Command{
	Use:   "llmshadow",
	Short: "Mirror and track websites' LLM-targeted content",
	Long:  "A tool that discovers, downloads, and maintains local mirrors of websites' LLM-targeted content with change tracking.",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootDir, "dir", "d", defaultDir(), "workspace directory")
	rootCmd.PersistentFlags().BoolVar(&respectRobots, "respect-robots", false, "respect robots.txt AI crawler directives")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output results as JSON")
}

// defaultDir returns the workspace directory, checking LLMSHADOW_DIR env var
// first, then falling back to ~/.config/llmshadow.
func defaultDir() string {
	if dir := os.Getenv("LLMSHADOW_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "llmshadow")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newEngine creates an Engine from the current flags.
func newEngine() (*engine.Engine, error) {
	// Ensure workspace directory exists
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("creating workspace dir: %w", err)
	}
	e, err := engine.New(rootDir, engine.Options{
		RespectRobots: respectRobots,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing: %w", err)
	}
	return e, nil
}

// printJSON marshals v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
