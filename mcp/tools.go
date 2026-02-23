package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dmoose/doctrove/internal/engine"
	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// --- trove_discover ---

func discoverTool() gomcp.Tool {
	return gomcp.NewTool("trove_discover",
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

// --- trove_scan ---

func scanTool() gomcp.Tool {
	return gomcp.NewTool("trove_scan",
		gomcp.WithDescription("Add a site and download its LLM content (init + sync)"),
		gomcp.WithString("url",
			gomcp.Required(),
			gomcp.Description("The base URL to track and sync (e.g. https://stripe.com)"),
		),
		gomcp.WithString("content_types",
			gomcp.Description("Comma-separated content types to sync (e.g. 'llms-txt,llms-full-txt'). Omit to sync all discovered content."),
		),
	)
}

func scanHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		url := stringArg(req, "url", "")
		if url == "" {
			return gomcp.NewToolResultError("url is required"), nil
		}

		contentTypes := stringArg(req, "content_types", "")

		info, err := e.Init(ctx, url)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("init: %v", err)), nil
		}

		var syncResult *engine.SyncResult
		if contentTypes != "" {
			syncResult, err = e.SyncWithContentTypes(ctx, info.Domain, contentTypes)
		} else {
			syncResult, err = e.Sync(ctx, info.Domain)
		}
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("sync: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"domain":    syncResult.Domain,
			"added":     len(syncResult.Added),
			"unchanged": len(syncResult.Unchanged),
			"skipped":   len(syncResult.Skipped),
			"errors":    syncResult.Errors,
			"sync_time": syncResult.SyncTime,
			"committed": syncResult.Committed,
		})
	}
}

// --- trove_search ---

func searchTool() gomcp.Tool {
	return gomcp.NewTool("trove_search",
		gomcp.WithDescription("Full-text search across all mirrored LLM content. Results include summaries when available (submitted via trove_summarize) — check these before reading full files. Recommended workflow: search → check summaries → trove_outline for structure → trove_read with section filter for targeted content."),
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
		gomcp.WithString("category",
			gomcp.Description("Filter by page category: api-reference, tutorial, guide, spec, changelog, marketing, legal, community, context7, index, other"),
		),
		gomcp.WithNumber("limit",
			gomcp.Description("Max number of results (default 20)"),
		),
		gomcp.WithNumber("offset",
			gomcp.Description("Skip first N results for pagination (default 0)"),
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
			stringArg(req, "category", ""),
			intArg(req, "limit", 20),
			intArg(req, "offset", 0),
		)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(hits)
	}
}

// --- trove_search_full ---

func searchFullTool() gomcp.Tool {
	return gomcp.NewTool("trove_search_full",
		gomcp.WithDescription("Search and return the full content of the best matching file. WARNING: can return very large results (100KB+). Prefer trove_search → trove_outline → trove_read with section filter for context-efficient access. Use this only when you genuinely need the entire file. After reading, call trove_summarize to cache a summary for future agents."),
		gomcp.WithString("query",
			gomcp.Required(),
			gomcp.Description("Search query"),
		),
		gomcp.WithString("site",
			gomcp.Description("Filter to a specific domain"),
		),
		gomcp.WithString("content_type",
			gomcp.Description("Filter by content type"),
		),
		gomcp.WithString("category",
			gomcp.Description("Filter by page category: api-reference, tutorial, guide, spec, changelog, marketing, legal, community, context7, index, other"),
		),
	)
}

func searchFullHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := stringArg(req, "query", "")
		if query == "" {
			return gomcp.NewToolResultError("query is required"), nil
		}

		result, err := e.SearchFull(ctx, query,
			stringArg(req, "site", ""),
			stringArg(req, "content_type", ""),
			stringArg(req, "category", ""),
		)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(result)
	}
}

// --- trove_list ---

func listTool() gomcp.Tool {
	return gomcp.NewTool("trove_list",
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

// --- trove_read ---

func readTool() gomcp.Tool {
	return gomcp.NewTool("trove_read",
		gomcp.WithDescription("Read a mirrored file by domain and path. Use 'section' to read just one heading's content instead of the whole file (saves context). Use trove_outline first to see available sections. After reading a large file, call trove_summarize to cache a summary so future agents can avoid re-reading it."),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithString("path",
			gomcp.Required(),
			gomcp.Description("The URL path of the file (e.g. /llms.txt)"),
		),
		gomcp.WithString("section",
			gomcp.Description("Read only the section matching this heading (case-insensitive substring match). Returns content from that heading to the next heading of same/higher level."),
		),
		gomcp.WithNumber("max_lines",
			gomcp.Description("Limit output to first N lines (useful for previewing large files)"),
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

		section := stringArg(req, "section", "")
		maxLines := intArg(req, "max_lines", 0)

		content, err := e.ReadSection(ctx, site, path, section, maxLines)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("reading %s%s: %v", site, path, err)), nil
		}

		lastSync := "unknown"
		if siteCfg, ok := e.Config.Sites[site]; ok && !siteCfg.LastSync.IsZero() {
			lastSync = siteCfg.LastSync.Format("2006-01-02T15:04:05Z07:00")
		}

		summary, _, _ := e.Index.GetSummary(site, path)

		result := map[string]any{
			"domain":    site,
			"path":      path,
			"size":      len(content),
			"last_sync": lastSync,
			"content":   content,
		}
		if summary != "" {
			result["summary"] = summary
		}
		if section != "" {
			result["section"] = section
		}

		return jsonResult(result)
	}
}

// --- trove_status ---

func statusTool() gomcp.Tool {
	return gomcp.NewTool("trove_status",
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

// --- trove_diff ---

func diffTool() gomcp.Tool {
	return gomcp.NewTool("trove_diff",
		gomcp.WithDescription("Show content changes between git refs (defaults to last change)"),
		gomcp.WithString("from",
			gomcp.Description("Start ref (e.g. HEAD~2). Omit for parent of 'to'"),
		),
		gomcp.WithString("to",
			gomcp.Description("End ref (default HEAD)"),
		),
	)
}

const maxDiffBytes = 50_000

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

		if len(diff) > maxDiffBytes {
			diff = diff[:maxDiffBytes] + fmt.Sprintf(
				"\n\n… truncated (%d bytes total, showing first %d). Use from/to refs to narrow the range.",
				len(diff), maxDiffBytes,
			)
		}
		return gomcp.NewToolResultText(diff), nil
	}
}

// --- trove_history ---

func historyTool() gomcp.Tool {
	return gomcp.NewTool("trove_history",
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

// --- trove_list_files ---

func listFilesTool() gomcp.Tool {
	return gomcp.NewTool("trove_list_files",
		gomcp.WithDescription("List mirrored files for a site with path, size, content type, and category. Use limit/offset to paginate large sites. For exploring what a site contains, try trove_catalog first (lighter), then list_files for detail."),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithNumber("limit",
			gomcp.Description("Max files to return (default 100, 0 = all)"),
		),
		gomcp.WithNumber("offset",
			gomcp.Description("Skip first N files for pagination (default 0)"),
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

		offset := intArg(req, "offset", 0)
		limit := intArg(req, "limit", 100)

		if offset > 0 {
			if offset >= len(files) {
				files = nil
			} else {
				files = files[offset:]
			}
		}
		if limit > 0 && limit < len(files) {
			files = files[:limit]
		}

		return jsonResult(files)
	}
}

// --- trove_remove ---

func removeTool() gomcp.Tool {
	return gomcp.NewTool("trove_remove",
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

// --- trove_catalog ---

func catalogTool() gomcp.Tool {
	return gomcp.NewTool("trove_catalog",
		gomcp.WithDescription("Get a compact summary of all tracked sites with titles, descriptions, and topic areas — use this to find which site has docs for a topic"),
	)
}

func catalogHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		entries, err := e.Catalog(ctx)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(entries)
	}
}

// --- trove_stats ---

func statsTool() gomcp.Tool {
	return gomcp.NewTool("trove_stats",
		gomcp.WithDescription("Get workspace statistics: total sites, files, disk usage, and sync freshness"),
	)
}

func statsHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		stats, err := e.Stats(ctx)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(stats)
	}
}

// --- trove_tag ---

func tagTool() gomcp.Tool {
	return gomcp.NewTool("trove_tag",
		gomcp.WithDescription("Override the category for a mirrored file. This is a persistent correction — it survives re-syncs and helps all future agents filter search results accurately. If you notice a page is miscategorized (e.g. a spec page tagged as marketing), fix it now so future searches work better."),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithString("path",
			gomcp.Required(),
			gomcp.Description("The URL path of the file (e.g. /docs/api.md)"),
		),
		gomcp.WithString("category",
			gomcp.Required(),
			gomcp.Description("New category: api-reference, tutorial, guide, spec, changelog, marketing, legal, community, index, other"),
		),
	)
}

func tagHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		path := stringArg(req, "path", "")
		category := stringArg(req, "category", "")
		if site == "" || path == "" || category == "" {
			return gomcp.NewToolResultError("site, path, and category are required"), nil
		}

		if err := e.Tag(ctx, site, path, category); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(map[string]string{
			"site":     site,
			"path":     path,
			"category": category,
			"status":   "updated",
		})
	}
}

// --- trove_check ---

func checkTool() gomcp.Tool {
	return gomcp.NewTool("trove_check",
		gomcp.WithDescription("Dry-run: probe a tracked site for available content without downloading — use this to preview what a sync would fetch"),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain to check (e.g. stripe.com)"),
		),
	)
}

func checkHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		result, err := e.Check(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(result)
	}
}

// --- trove_refresh ---

func refreshTool() gomcp.Tool {
	return gomcp.NewTool("trove_refresh",
		gomcp.WithDescription("Re-sync a tracked site to pick up changes — uses cached ETags to skip unchanged files"),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain to refresh (e.g. modelcontextprotocol.io)"),
		),
	)
}

func refreshHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		result, err := e.Refresh(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(map[string]any{
			"domain":    result.Domain,
			"added":     len(result.Added),
			"unchanged": len(result.Unchanged),
			"skipped":   len(result.Skipped),
			"errors":    result.Errors,
			"sync_time": result.SyncTime,
		})
	}
}

// --- trove_outline ---

func outlineTool() gomcp.Tool {
	return gomcp.NewTool("trove_outline",
		gomcp.WithDescription("Get the heading structure (table of contents) of a mirrored file with section sizes. Use this BEFORE trove_read to identify which section you need, then read just that section. If a summary exists (from a previous trove_summarize call), it is included — check if the summary answers your question before reading the full content."),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithString("path",
			gomcp.Required(),
			gomcp.Description("The URL path of the file (e.g. /llms.txt)"),
		),
		gomcp.WithNumber("max_depth",
			gomcp.Description("Max heading depth to include (1-6, default 3). Use 0 for all levels."),
		),
		gomcp.WithNumber("max_sections",
			gomcp.Description("Max sections to return (default 100). Use 0 for unlimited."),
		),
	)
}

func outlineHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		path := stringArg(req, "path", "")
		if site == "" || path == "" {
			return gomcp.NewToolResultError("site and path are required"), nil
		}
		maxDepth := intArg(req, "max_depth", 3)
		maxSections := intArg(req, "max_sections", 100)

		result, err := e.Outline(ctx, site, path, maxDepth, maxSections)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(result)
	}
}

// --- trove_summarize ---

func summarizeTool() gomcp.Tool {
	return gomcp.NewTool("trove_summarize",
		gomcp.WithDescription("Store a summary for a file you've read. This is an important feedback mechanism: after reading a large file, submit a concise summary (key topics, what the file covers, when to use it). Future agents will see this summary in search results and trove_outline, letting them decide whether to read the full content — saving significant context. Summaries should be 2-5 sentences covering: what the document is about, key topics/APIs it covers, and when an agent would need the full content."),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithString("path",
			gomcp.Required(),
			gomcp.Description("The URL path of the file (e.g. /docs/api.md)"),
		),
		gomcp.WithString("summary",
			gomcp.Required(),
			gomcp.Description("A concise summary of the file's content (2-5 sentences). Cover: what it's about, key topics, when to read the full content."),
		),
	)
}

func summarizeHandler(e *engine.Engine) server_handler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := stringArg(req, "site", "")
		path := stringArg(req, "path", "")
		summary := stringArg(req, "summary", "")
		if site == "" || path == "" || summary == "" {
			return gomcp.NewToolResultError("site, path, and summary are required"), nil
		}

		if err := e.Summarize(ctx, site, path, summary); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(map[string]string{
			"site":   site,
			"path":   path,
			"status": "summary saved",
		})
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
