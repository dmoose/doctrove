package mcp

import (
	"fmt"

	"github.com/dmoose/llmshadow/internal/engine"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates an MCP server wired to the given engine.
func NewServer(e *engine.Engine) *server.MCPServer {
	s := server.NewMCPServer(
		"llmshadow",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	registerTools(s, e)
	return s
}

// Serve starts the MCP server on stdio.
func Serve(e *engine.Engine) error {
	s := NewServer(e)
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}

func registerTools(s *server.MCPServer, e *engine.Engine) {
	s.AddTool(discoverTool(), discoverHandler(e))
	s.AddTool(scanTool(), scanHandler(e))
	s.AddTool(searchTool(), searchHandler(e))
	s.AddTool(searchFullTool(), searchFullHandler(e))
	s.AddTool(listTool(), listHandler(e))
	s.AddTool(readTool(), readHandler(e))
	s.AddTool(statusTool(), statusHandler(e))
	s.AddTool(diffTool(), diffHandler(e))
	s.AddTool(historyTool(), historyHandler(e))
	s.AddTool(listFilesTool(), listFilesHandler(e))
	s.AddTool(removeTool(), removeHandler(e))
	s.AddTool(catalogTool(), catalogHandler(e))
	s.AddTool(statsTool(), statsHandler(e))
}

// helper to pull a bool arg with a default
func boolArg(req gomcp.CallToolRequest, key string, def bool) bool {
	v, ok := req.GetArguments()[key]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// helper to pull a string arg with a default
func stringArg(req gomcp.CallToolRequest, key, def string) string {
	v, ok := req.GetArguments()[key]
	if !ok {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return s
}

// helper to pull an int arg with a default
func intArg(req gomcp.CallToolRequest, key string, def int) int {
	v, ok := req.GetArguments()[key]
	if !ok {
		return def
	}
	f, ok := v.(float64) // JSON numbers are float64
	if !ok {
		return def
	}
	return int(f)
}
