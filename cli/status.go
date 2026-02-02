package cli

import (
	"fmt"

	"github.com/dmoose/llmshadow/internal/engine"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [site]",
	Short: "Show status of tracked sites",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}

		if len(args) == 1 {
			info, err := e.Status(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printSiteInfo(info)
			return nil
		}

		sites, err := e.List(cmd.Context())
		if err != nil {
			return err
		}
		if len(sites) == 0 {
			fmt.Println("No sites tracked. Run 'llmshadow init <url>' first.")
			return nil
		}
		for _, s := range sites {
			printSiteInfo(&s)
			fmt.Println()
		}
		return nil
	},
}

func printSiteInfo(info *engine.SiteInfo) {
	fmt.Printf("%s (%s)\n", info.Domain, info.URL)
	fmt.Printf("  Files: %d\n", info.FileCount)
	if !info.LastSync.IsZero() {
		fmt.Printf("  Last sync: %s\n", info.LastSync.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("  Last sync: never\n")
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
