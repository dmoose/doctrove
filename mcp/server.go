package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dmoose/llmshadow/internal/engine"
	"github.com/dmoose/llmshadow/internal/events"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// defaultAgentID derives a session identifier from the caller's working
// directory (last path component) plus the first 6 hex digits of the PID.
// Example: "my-project:00a3f1". An explicit agent_id in a tool call overrides.
func defaultAgentID() string {
	dir, _ := os.Getwd()
	base := filepath.Base(dir)
	pid := strconv.FormatInt(int64(os.Getpid()), 16)
	if len(pid) > 6 {
		pid = pid[:6]
	}
	return base + ":" + pid
}

// NewServer creates an MCP server wired to the given engine.
func NewServer(e *engine.Engine) *server.MCPServer {
	s := server.NewMCPServer(
		"llmshadow",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	sessionID := defaultAgentID()
	e.Events.SetAgentID(sessionID)
	registerTools(s, e, sessionID)
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

func registerTools(s *server.MCPServer, e *engine.Engine, sessionID string) {
	add := func(tool gomcp.Tool, handler server_handler) {
		s.AddTool(tool, traced(e, tool.Name, sessionID, handler))
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
	add(tagTool(), tagHandler(e))
	add(refreshTool(), refreshHandler(e))
}

// traced wraps a tool handler to emit events to the relay.
// sessionID is the default agent identity (cwd:pid); an explicit agent_id
// argument in the tool call overrides it.
func traced(e *engine.Engine, toolName, sessionID string, handler server_handler) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		start := time.Now()
		agentID := stringArg(req, "agent_id", sessionID)

		result, err := handler(ctx, req)

		level := "info"
		if err != nil || (result != nil && result.IsError) {
			level = "error"
		}

		e.Events.EmitFull(events.Event{
			Channel:    "mcp",
			Action:     toolName,
			Level:      level,
			AgentID:    agentID,
			DurationMS: time.Since(start).Milliseconds(),
			Data:       req.GetArguments(),
		})
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
