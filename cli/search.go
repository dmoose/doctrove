package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	searchSite        string
	searchContentType string
	searchLimit       int
	searchRebuild     bool
	searchFull        bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across mirrored content",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer e.Close()

		if searchRebuild {
			fmt.Println("Rebuilding search index...")
			if err := e.RebuildIndex(cmd.Context()); err != nil {
				return fmt.Errorf("rebuilding index: %w", err)
			}
		}

		query := args[0]

		// --full: return complete content of best match
		if searchFull {
			result, err := e.SearchFull(cmd.Context(), query, searchSite, searchContentType)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(result)
			}
			if result.Content == "" {
				fmt.Println("No results.")
				if result.Suggestion != "" {
					fmt.Println(result.Suggestion)
				}
				return nil
			}
			fmt.Printf("--- %s %s [%s] ---\n\n", result.Domain, result.Path, result.ContentType)
			fmt.Print(result.Content)
			return nil
		}

		sr, err := e.Search(cmd.Context(), query, searchSite, searchContentType, searchLimit)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(sr)
		}

		if len(sr.Hits) == 0 {
			fmt.Println("No results.")
			if sr.Suggestion != "" {
				fmt.Println(sr.Suggestion)
			}
			return nil
		}

		for _, h := range sr.Hits {
			fmt.Printf("%s %s [%s]\n", h.Domain, h.Path, h.ContentType)
			fmt.Printf("  %s\n\n", h.Snippet)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchSite, "site", "", "filter to a specific domain")
	searchCmd.Flags().StringVar(&searchContentType, "type", "", "filter by content type: llms-txt, companion, etc.")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 20, "max results")
	searchCmd.Flags().BoolVar(&searchRebuild, "rebuild", false, "rebuild search index before searching")
	searchCmd.Flags().BoolVar(&searchFull, "full", false, "return full content of the best match")
	rootCmd.AddCommand(searchCmd)
}
