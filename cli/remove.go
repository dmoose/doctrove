package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var keepFiles bool

var removeCmd = &cobra.Command{
	Use:   "remove <site>",
	Short: "Stop tracking a site",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		domain := args[0]
		if err := e.Remove(cmd.Context(), domain, keepFiles); err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(map[string]string{"removed": domain})
		}

		fmt.Printf("Removed %s\n", domain)
		if keepFiles {
			fmt.Println("  Files kept on disk.")
		}
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&keepFiles, "keep-files", false, "keep mirrored files on disk")
	rootCmd.AddCommand(removeCmd)
}
