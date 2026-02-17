package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <url>",
	Short: "Add a site to track",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		info, err := e.Init(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(info)
		}

		fmt.Printf("Tracking %s (%s)\n", info.Domain, info.URL)
		fmt.Printf("Discovered %d LLM content files\n", info.FileCount)
		fmt.Println("Run 'llmshadow sync' to download content.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
