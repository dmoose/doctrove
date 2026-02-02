package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var listFormat string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked sites",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}

		sites, err := e.List(cmd.Context())
		if err != nil {
			return err
		}

		if len(sites) == 0 {
			fmt.Println("No sites tracked. Run 'llmshadow init <url>' first.")
			return nil
		}

		if listFormat == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(sites)
		}

		for _, s := range sites {
			sync := "never"
			if !s.LastSync.IsZero() {
				sync = s.LastSync.Format("2006-01-02 15:04")
			}
			fmt.Printf("%-30s %3d files  synced: %s\n", s.Domain, s.FileCount, sync)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listFormat, "format", "table", "output format: table, json")
	rootCmd.AddCommand(listCmd)
}
