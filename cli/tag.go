package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag <site> <path> <category>",
	Short: "Override the category for a mirrored file",
	Long:  "Set the category for a page (api-reference, tutorial, guide, spec, changelog, marketing, legal, community, other).",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		site, path, category := args[0], args[1], args[2]

		if err := e.Tag(cmd.Context(), site, path, category); err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(map[string]string{
				"site":     site,
				"path":     path,
				"category": category,
				"status":   "updated",
			})
		}
		fmt.Printf("Tagged %s%s as %s\n", site, path, category)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
}
