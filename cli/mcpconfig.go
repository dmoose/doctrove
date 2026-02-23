package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var mcpConfigCmd = &cobra.Command{
	Use:   "mcp-config",
	Short: "Output MCP server configuration for agent tools",
	Long:  "Prints the JSON config snippet to add doctrove as an MCP server in Claude Code, Cursor, or other MCP-compatible agents.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		bin, err := exec.LookPath("doctrove")
		if err != nil {
			bin = "doctrove"
		}

		config := fmt.Sprintf(`{
  "mcpServers": {
    "doctrove": {
      "command": "%s",
      "args": ["mcp", "--dir", "%s"]
    }
  }
}`, bin, rootDir)

		fmt.Println(config)
		fmt.Println()
		fmt.Println("Add the mcpServers entry to:")
		fmt.Printf("  Claude Code (user):    %s\n", claudeCodeConfigPath())
		fmt.Println("  Claude Code (project): .mcp.json (in project root)")
		fmt.Println("  Cursor:                .cursor/mcp.json (in project root)")
		fmt.Println()
		fmt.Println("Or merge into your existing config if you already have mcpServers defined.")
		return nil
	},
}

func claudeCodeConfigPath() string {
	// Path is the same on all platforms
	return filepath.Join("~", ".claude.json")
}

func init() {
	rootCmd.AddCommand(mcpConfigCmd)
}
