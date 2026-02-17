package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check <site>",
	Short: "Dry-run: show what content is available without downloading",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		result, err := e.Check(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(result)
		}

		if len(result.Available) == 0 {
			fmt.Printf("No LLM content found for %s\n", result.Domain)
			return nil
		}

		fmt.Printf("%s: %d files available\n", result.Domain, len(result.Available))
		for _, p := range result.Available {
			fmt.Printf("  %s\n", p)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
