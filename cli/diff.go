package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [from] [to]",
	Short: "Show content changes between syncs",
	Long:  "Show diff between two git refs. Defaults to showing the last change (HEAD~1..HEAD).",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		from := ""
		to := "HEAD"
		if len(args) >= 1 {
			from = args[0]
		}
		if len(args) >= 2 {
			to = args[1]
		}

		diff, err := e.Diff(cmd.Context(), from, to)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(map[string]string{"diff": diff})
		}

		if diff == "" {
			fmt.Println("No changes.")
			return nil
		}

		fmt.Print(diff)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
