package mcp

import (
	"context"
	"fmt"
	"time"

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
	add := func(tool gomcp.Tool, handler server_handler) {
		s.AddTool(tool, traced(e, tool.Name, handler))
	}
	add(discoverTool(), discoverHandler(e))
	add(scanTool(), scanHandler(e))
	add(searchTool(), searchHandler(e))
	add(searchFullTool(), searchFullHandler(e))
	add(listTool(), listHandler(e))
	add(readTool(), readHandler(e))
	add(statusTool(), statusHandler(e))
	add(diffTool(), diffHandler(e))
	add(historyTool(), historyHandler(e))
	add(listFilesTool(), listFilesHandler(e))
	add(removeTool(), removeHandler(e))
	add(catalogTool(), catalogHandler(e))
	add(statsTool(), statsHandler(e))
}

// traced wraps a tool handler to emit events to the relay.
func traced(e *engine.Engine, toolName string, handler server_handler) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		start := time.Now()
		agentID := stringArg(req, "agent_id", "")

		result, err := handler(ctx, req)

		data := map[string]any{
			"tool":        toolName,
			"duration_ms": time.Since(start).Milliseconds(),
			"args":        req.GetArguments(),
		}
		if result != nil && result.IsError {
			data["error"] = true
		}

		e.Events.Emit(toolName, agentID, data)
		return result, err
	}
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
