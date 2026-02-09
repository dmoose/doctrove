package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show workspace statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}

		stats, err := e.Stats(cmd.Context())
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(stats)
		}

		fmt.Printf("Sites: %d  Files: %d  Size: %s\n", stats.TotalSites, stats.TotalFiles, stats.TotalSizeHuman)
		if !stats.OldestSync.IsZero() {
			fmt.Printf("Oldest sync: %s  Newest sync: %s\n",
				stats.OldestSync.Format("2006-01-02 15:04"),
				stats.NewestSync.Format("2006-01-02 15:04"))
		}
		fmt.Println()

		for _, s := range stats.SiteStats {
			age := "never synced"
			if s.Age != "" {
				age = s.Age
			}
			fmt.Printf("  %-30s %3d files  %8s  %s\n", s.Domain, s.FileCount, s.SizeHuman, age)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
