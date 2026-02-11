package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var grabCmd = &cobra.Command{
	Use:   "grab <url>",
	Short: "Discover, track, and sync a site in one step",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer e.Close()

		info, err := e.Init(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		result, err := e.Sync(cmd.Context(), info.Domain)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(result)
		}

		fmt.Printf("Grabbed %s: %d files synced\n", info.Domain, len(result.Added))
		for _, f := range result.Added {
			fmt.Printf("  + %s\n", f)
		}
		for _, e := range result.Errors {
			fmt.Printf("  ! %s\n", e)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(grabCmd)
}
