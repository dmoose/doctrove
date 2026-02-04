package cli

import (
	mcpserver "github.com/dmoose/llmshadow/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio transport)",
	Long:  "Starts an MCP server over stdin/stdout for agent integration.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		e, err := newEngine()
		if err != nil {
			return err
		}
		defer e.Close()

		return mcpserver.Serve(e)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
