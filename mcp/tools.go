package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dmoose/llmshadow/internal/engine"
	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// --- shadow_discover ---

func discoverTool() gomcp.Tool {
	return gomcp.NewTool("shadow_discover",
		gomcp.WithDescription("Probe a URL for LLM-targeted content (llms.txt, companions) without saving anything locally"),
		gomcp.WithString("url",
			gomcp.Required(),
			gomcp.Description("The base URL to probe (e.g. https://stripe.com)"),
		),
	)
}

func discoverHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		url := stringArg(req, "url", "")
		if url == "" {
			return gomcp.NewToolResultError("url is required"), nil
		}

		result, err := e.Discover(ctx, url)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(result)
	}
}

// --- shadow_scan ---

func scanTool() gomcp.Tool {
	return gomcp.NewTool("shadow_scan",
		gomcp.WithDescription("Add a site and download its LLM content (init + sync)"),
		gomcp.WithString("url",
			gomcp.Required(),
			gomcp.Description("The base URL to track and sync (e.g. https://stripe.com)"),
		),
	)
}

func scanHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		url := stringArg(req, "url", "")
		if url == "" {
			return gomcp.NewToolResultError("url is required"), nil
		}

		info, err := e.Init(ctx, url)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("init: %v", err)), nil
		}

		syncResult, err := e.Sync(ctx, info.Domain)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("sync: %v", err)), nil
		}

		return jsonResult(syncResult)
	}
}

// --- shadow_search ---

func searchTool() gomcp.Tool {
	return gomcp.NewTool("shadow_search",
		gomcp.WithDescription("Full-text search across all mirrored LLM content"),
		gomcp.WithString("query",
			gomcp.Required(),
			gomcp.Description("Search query (supports FTS5 syntax)"),
		),
		gomcp.WithString("site",
			gomcp.Description("Filter results to a specific domain"),
		),
		gomcp.WithString("content_type",
			gomcp.Description("Filter by content type: llms-txt, llms-full-txt, ai-txt, companion"),
		),
		gomcp.WithNumber("limit",
			gomcp.Description("Max number of results (default 20)"),
		),
	)
}

func searchHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := stringArg(req, "query", "")
		if query == "" {
			return gomcp.NewToolResultError("query is required"), nil
		}

		hits, err := e.Search(ctx, query,
			stringArg(req, "site", ""),
			stringArg(req, "content_type", ""),
			intArg(req, "limit", 20),
		)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(hits)
	}
}

// --- shadow_list ---

func listTool() gomcp.Tool {
	return gomcp.NewTool("shadow_list",
		gomcp.WithDescription("List all tracked sites with sync status and file counts"),
	)
}

func listHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		sites, err := e.List(ctx)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(sites)
	}
}

// --- shadow_read ---

func readTool() gomcp.Tool {
	return gomcp.NewTool("shadow_read",
		gomcp.WithDescription("Read a specific mirrored file by domain and path"),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithString("path",
			gomcp.Required(),
			gomcp.Description("The URL path of the file (e.g. /llms.txt)"),
		),
	)
}

func readHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		path := stringArg(req, "path", "")
		if site == "" || path == "" {
			return gomcp.NewToolResultError("site and path are required"), nil
		}

		data, err := e.Store.ReadContent(site, path)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("reading %s%s: %v", site, path, err)), nil
		}

		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- shadow_status ---

func statusTool() gomcp.Tool {
	return gomcp.NewTool("shadow_status",
		gomcp.WithDescription("Get sync status and file count for a tracked site"),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
	)
}

func statusHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		info, err := e.Status(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(info)
	}
}

// --- shadow_diff ---

func diffTool() gomcp.Tool {
	return gomcp.NewTool("shadow_diff",
		gomcp.WithDescription("Show content changes between git refs (defaults to last change)"),
		gomcp.WithString("from",
			gomcp.Description("Start ref (e.g. HEAD~2). Omit for parent of 'to'"),
		),
		gomcp.WithString("to",
			gomcp.Description("End ref (default HEAD)"),
		),
	)
}

func diffHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		from := stringArg(req, "from", "")
		to := stringArg(req, "to", "HEAD")

		diff, err := e.Diff(ctx, from, to)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		if diff == "" {
			return gomcp.NewToolResultText("No changes."), nil
		}
		return gomcp.NewToolResultText(diff), nil
	}
}

// --- shadow_history ---

func historyTool() gomcp.Tool {
	return gomcp.NewTool("shadow_history",
		gomcp.WithDescription("Show git change history, optionally filtered to a site"),
		gomcp.WithString("site",
			gomcp.Description("Filter to a specific domain"),
		),
		gomcp.WithNumber("limit",
			gomcp.Description("Max entries to return (default 20)"),
		),
	)
}

func historyHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		limit := intArg(req, "limit", 20)

		entries, err := e.History(ctx, site, limit)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(entries)
	}
}

// --- shadow_list_files ---

func listFilesTool() gomcp.Tool {
	return gomcp.NewTool("shadow_list_files",
		gomcp.WithDescription("List all mirrored files for a site with path, size, and content type"),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
	)
}

func listFilesHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		files, err := e.ListFiles(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(files)
	}
}

// --- shadow_remove ---

func removeTool() gomcp.Tool {
	return gomcp.NewTool("shadow_remove",
		gomcp.WithDescription("Stop tracking a site, remove files and index entries"),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain to remove (e.g. stripe.com)"),
		),
		gomcp.WithBoolean("keep_files",
			gomcp.Description("Keep mirrored files on disk (default false)"),
		),
	)
}

func removeHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		keepFiles := boolArg(req, "keep_files", false)
		if err := e.Remove(ctx, site, keepFiles); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(map[string]string{"removed": site})
	}
}

// --- helpers ---

type server_handler = func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error)

func jsonResult(v any) (*gomcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return gomcp.NewToolResultText(string(data)), nil
}
