package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover <url>",
	Short: "Probe a URL for LLM content without tracking it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		result, err := e.Discover(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(result)
		}

		if len(result.Files) == 0 {
			fmt.Printf("No LLM content found at %s\n", result.BaseURL)
			return nil
		}

		fmt.Printf("%s: %d files found\n", result.Domain, len(result.Files))
		for _, f := range result.Files {
			fmt.Printf("  %-15s %s (%d bytes, %s)\n", f.ContentType, f.Path, f.Size, f.FoundVia)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
