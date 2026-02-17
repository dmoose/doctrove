package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	historyLimit int
	historySince string
)

var historyCmd = &cobra.Command{
	Use:   "history [site]",
	Short: "Show change history from git",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		site := ""
		if len(args) == 1 {
			site = args[0]
		}

		entries, err := e.History(cmd.Context(), site, historyLimit)
		if err != nil {
			return err
		}

		// Filter by --since if specified
		if historySince != "" {
			since, err := parseSince(historySince)
			if err != nil {
				return fmt.Errorf("invalid --since value: %w", err)
			}
			filtered := entries[:0]
			for _, entry := range entries {
				if entry.When.After(since) {
					filtered = append(filtered, entry)
				}
			}
			entries = filtered
		}

		if jsonOutput {
			return printJSON(entries)
		}

		if len(entries) == 0 {
			fmt.Println("No history.")
			return nil
		}

		for _, entry := range entries {
			fmt.Printf("%s  %s  %s\n", entry.Hash, entry.When.Format("2006-01-02 15:04"), entry.Message)
		}
		return nil
	},
}

// parseSince parses a duration (e.g. "7d", "24h") or a date string.
func parseSince(s string) (time.Time, error) {
	// Try duration-style: "7d", "24h", "30m"
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(s[:len(s)-1])
		if err == nil {
			return time.Now().AddDate(0, 0, -days), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	// Try date formats
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("expected duration (e.g. 7d, 24h) or date (e.g. 2026-03-10)")
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "number of entries to show")
	historyCmd.Flags().StringVar(&historySince, "since", "", "show entries since duration (7d, 24h) or date (2026-03-10)")
	rootCmd.AddCommand(historyCmd)
}
