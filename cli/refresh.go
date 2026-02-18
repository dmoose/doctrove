package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var refreshAll bool

var refreshCmd = &cobra.Command{
	Use:   "refresh [site]",
	Short: "Re-sync tracked sites, skipping unchanged files via ETag caching",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		if refreshAll || len(args) == 0 {
			results, err := e.SyncAll(cmd.Context())
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No sites tracked. Run 'llmshadow grab <url>' first.")
				return nil
			}
			if jsonOutput {
				return printJSON(results)
			}
			for _, r := range results {
				printRefreshResult(r.Domain, r.Added, r.Unchanged, r.Errors)
			}
			return nil
		}

		r, err := e.Refresh(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(r)
		}
		printRefreshResult(r.Domain, r.Added, r.Unchanged, r.Errors)
		return nil
	},
}

func printRefreshResult(domain string, added, unchanged, errors []string) {
	fmt.Printf("Refreshed %s: %d updated, %d unchanged\n", domain, len(added), len(unchanged))
	for _, f := range added {
		fmt.Printf("  + %s\n", f)
	}
	for _, e := range errors {
		fmt.Printf("  ! %s\n", e)
	}
}

func init() {
	refreshCmd.Flags().BoolVar(&refreshAll, "all", false, "refresh all tracked sites")
	rootCmd.AddCommand(refreshCmd)
}
