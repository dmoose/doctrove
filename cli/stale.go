package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var staleThreshold string

var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "Show sites that haven't been synced recently",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer func() { _ = e.Close() }()

		threshold, err := parseStaleDuration(staleThreshold)
		if err != nil {
			return err
		}

		stale, err := e.Stale(cmd.Context(), threshold)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(stale)
		}

		if len(stale) == 0 {
			fmt.Printf("All sites synced within %s.\n", staleThreshold)
			return nil
		}

		fmt.Printf("%d stale sites (not synced in %s):\n", len(stale), staleThreshold)
		for _, s := range stale {
			age := "never synced"
			if s.Age != "" {
				age = s.Age
			}
			fmt.Printf("  %-30s %s\n", s.Domain, age)
		}
		return nil
	},
}

func parseStaleDuration(s string) (time.Duration, error) {
	// Support "7d" style
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func init() {
	staleCmd.Flags().StringVar(&staleThreshold, "threshold", "7d", "consider stale after this duration (e.g. 7d, 24h)")
	rootCmd.AddCommand(staleCmd)
}
