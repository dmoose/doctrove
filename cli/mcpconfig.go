package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var mcpConfigCmd = &cobra.Command{
	Use:   "mcp-config",
	Short: "Output MCP server configuration for agent tools",
	Long:  "Prints the JSON config snippet to add llmshadow as an MCP server in Claude Code, Cursor, or other MCP-compatible agents.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		bin, err := exec.LookPath("llmshadow")
		if err != nil {
			bin = "llmshadow"
		}

		config := fmt.Sprintf(`{
  "mcpServers": {
    "llmshadow": {
      "command": "%s",
      "args": ["mcp", "--dir", "%s"]
    }
  }
}`, bin, rootDir)

		fmt.Println(config)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpConfigCmd)
}
