package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history [site]",
	Short: "Show change history from git",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}

		site := ""
		if len(args) == 1 {
			site = args[0]
		}

		entries, err := e.History(cmd.Context(), site, historyLimit)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No history yet.")
			return nil
		}

		for _, entry := range entries {
			fmt.Printf("%s  %s  %s\n", entry.Hash, entry.When.Format("2006-01-02 15:04"), entry.Message)
		}
		return nil
	},
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "number of entries to show")
	rootCmd.AddCommand(historyCmd)
}
