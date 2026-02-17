package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var catalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Show a summary of all tracked sites and their topics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		entries, err := e.Catalog(cmd.Context())
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(entries)
		}

		if len(entries) == 0 {
			fmt.Println("No sites tracked.")
			return nil
		}

		for _, entry := range entries {
			fmt.Printf("%s (%d files)\n", entry.Domain, entry.FileCount)
			if entry.Title != "" {
				fmt.Printf("  %s\n", entry.Title)
			}
			if entry.Description != "" {
				desc := entry.Description
				if len(desc) > 120 {
					desc = desc[:117] + "..."
				}
				fmt.Printf("  %s\n", desc)
			}
			if len(entry.Topics) > 0 {
				fmt.Printf("  Topics: %s\n", strings.Join(entry.Topics, ", "))
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(catalogCmd)
}
