package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncAll bool

var syncCmd = &cobra.Command{
	Use:   "sync [site]",
	Short: "Download/update LLM content for a site",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}

		if syncAll || len(args) == 0 {
			results, err := e.SyncAll(cmd.Context())
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No sites tracked. Run 'llmshadow init <url>' first.")
				return nil
			}
			if jsonOutput {
				return printJSON(results)
			}
			for _, r := range results {
				printSyncResult(r.Domain, r.Added, r.Errors)
			}
			return nil
		}

		r, err := e.Sync(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(r)
		}
		printSyncResult(r.Domain, r.Added, r.Errors)
		return nil
	},
}

func printSyncResult(domain string, added []string, errors []string) {
	fmt.Printf("Synced %s: %d files\n", domain, len(added))
	for _, f := range added {
		fmt.Printf("  + %s\n", f)
	}
	for _, e := range errors {
		fmt.Printf("  ! %s\n", e)
	}
}

func init() {
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "sync all tracked sites")
	rootCmd.AddCommand(syncCmd)
}
