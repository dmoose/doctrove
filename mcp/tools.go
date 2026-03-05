package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dmoose/doctrove/engine"
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

func discoverHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		url := StringArg(req, "url", "")
		if url == "" {
			return gomcp.NewToolResultError("url is required"), nil
		}

		result, err := e.Discover(ctx, url)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(result)
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

func scanHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		url := StringArg(req, "url", "")
		if url == "" {
			return gomcp.NewToolResultError("url is required"), nil
		}

		contentTypes := StringArg(req, "content_types", "")

		info, err := e.Init(ctx, url)
		if err != nil {
			// If already tracked, update content_types and re-sync instead of failing
			if strings.Contains(err.Error(), "already tracked") {
				domain := domainFromURL(url)
				if contentTypes != "" {
					e.Config.SetContentTypes(domain, contentTypes)
				} else {
					e.Config.SetContentTypes(domain, "")
				}
				var syncResult *engine.SyncResult
				if contentTypes != "" {
					syncResult, err = e.SyncWithContentTypes(ctx, domain, contentTypes)
				} else {
					syncResult, err = e.Sync(ctx, domain)
				}
				if err != nil {
					return gomcp.NewToolResultError(fmt.Sprintf("sync: %v", err)), nil
				}
				return scanResult(syncResult)
			}
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

		return scanResult(syncResult)
	}
}

func scanResult(syncResult *engine.SyncResult) (*gomcp.CallToolResult, error) {
	result := map[string]any{
		"domain":    syncResult.Domain,
		"added":     len(syncResult.Added),
		"updated":   len(syncResult.Updated),
		"unchanged": len(syncResult.Unchanged),
		"skipped":   len(syncResult.Skipped),
		"sync_time": syncResult.SyncTime,
		"committed": syncResult.Committed,
	}
	if len(syncResult.Errors) > 0 {
		result["warnings"] = len(syncResult.Errors)
		result["warning_details"] = syncResult.Errors
	}
	return JsonResult(result)
}

// domainFromURL extracts the domain from a URL string.
func domainFromURL(rawURL string) string {
	start := 0
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		start = idx + 3
	}
	end := len(rawURL)
	for i := start; i < len(rawURL); i++ {
		if rawURL[i] == '/' {
			end = i
			break
		}
	}
	return rawURL[start:end]
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
		gomcp.WithString("path",
			gomcp.Description("Filter to paths containing this substring (e.g. '/specification/' or '/api/')"),
		),
		gomcp.WithNumber("limit",
			gomcp.Description("Max number of results (default 20)"),
		),
		gomcp.WithNumber("offset",
			gomcp.Description("Skip first N results for pagination (default 0)"),
		),
	)
}

func searchHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := StringArg(req, "query", "")
		if query == "" {
			return gomcp.NewToolResultError("query is required"), nil
		}

		if cat := StringArg(req, "category", ""); cat != "" && !validCategories[cat] {
			return gomcp.NewToolResultError(invalidCategoryMsg(cat)), nil
		}

		hits, err := e.Search(ctx, query,
			StringArg(req, "site", ""),
			StringArg(req, "content_type", ""),
			StringArg(req, "category", ""),
			StringArg(req, "path", ""),
			IntArg(req, "limit", 20),
			IntArg(req, "offset", 0),
		)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(hits)
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

func searchFullHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		query := StringArg(req, "query", "")
		if query == "" {
			return gomcp.NewToolResultError("query is required"), nil
		}

		if cat := StringArg(req, "category", ""); cat != "" && !validCategories[cat] {
			return gomcp.NewToolResultError(invalidCategoryMsg(cat)), nil
		}

		result, err := e.SearchFull(ctx, query,
			StringArg(req, "site", ""),
			StringArg(req, "content_type", ""),
			StringArg(req, "category", ""),
		)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(result)
	}
}

// --- trove_list ---

func listTool() gomcp.Tool {
	return gomcp.NewTool("trove_list",
		gomcp.WithDescription("List all tracked sites with sync status and file counts"),
	)
}

func listHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		sites, err := e.List(ctx)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		if sites == nil {
			sites = []engine.SiteInfo{}
		}
		return JsonResult(sites)
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

func readHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		path := StringArg(req, "path", "")
		if site == "" || path == "" {
			return gomcp.NewToolResultError("site and path are required"), nil
		}

		section := StringArg(req, "section", "")
		maxLines := IntArg(req, "max_lines", 0)

		content, err := e.ReadSection(ctx, site, path, section, maxLines)
		if err != nil {
			return gomcp.NewToolResultError(sanitizeError(site, path, err)), nil
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

		return JsonResult(result)
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

func statusHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		info, err := e.Status(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(info)
	}
}

// --- trove_diff ---

func diffTool() gomcp.Tool {
	return gomcp.NewTool("trove_diff",
		gomcp.WithDescription("Show content changes between git refs (defaults to last change). Use stat=true to get a compact summary of changed files instead of full diff (recommended first to gauge size)."),
		gomcp.WithString("from",
			gomcp.Description("Start ref (e.g. HEAD~2). Omit for parent of 'to'"),
		),
		gomcp.WithString("to",
			gomcp.Description("End ref (default HEAD)"),
		),
		gomcp.WithString("since",
			gomcp.Description("Time-based range (e.g. '2h', '1d', '30m'). Alternative to from/to — finds the oldest commit within this duration and diffs from there."),
		),
		gomcp.WithBoolean("stat",
			gomcp.Description("If true, return only a file-level summary (names + line counts) instead of full diff"),
		),
	)
}

const maxDiffBytes = 50_000

func diffHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		from := StringArg(req, "from", "")
		to := StringArg(req, "to", "HEAD")
		since := StringArg(req, "since", "")
		stat := BoolArg(req, "stat", false)

		// If "since" is provided, resolve it to a git ref by walking the log.
		if since != "" && from == "" {
			dur, parseErr := parseDuration(since)
			if parseErr != nil {
				return gomcp.NewToolResultError(fmt.Sprintf("invalid since duration %q: %v", since, parseErr)), nil
			}
			cutoff := time.Now().Add(-dur)
			entries, logErr := e.History(ctx, "", 1000)
			if logErr != nil {
				return gomcp.NewToolResultError(logErr.Error()), nil
			}
			// Find the oldest commit newer than cutoff
			for i := len(entries) - 1; i >= 0; i-- {
				if entries[i].When.After(cutoff) {
					from = entries[i].Hash + "~1"
					break
				}
			}
			if from == "" {
				return gomcp.NewToolResultText(fmt.Sprintf("No changes in the last %s.", since)), nil
			}
		}

		diff, err := e.Diff(ctx, from, to)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		// Filter out internal metadata files — agents care about content changes
		diff = filterDiffToContent(diff)

		if diff == "" {
			return gomcp.NewToolResultText("No changes."), nil
		}

		if stat {
			return JsonResult(diffStat(diff))
		}

		if len(diff) > maxDiffBytes {
			diff = diff[:maxDiffBytes] + fmt.Sprintf(
				"\n\n… truncated (%d bytes total, showing first %d). Use stat=true for a summary, or from/to refs to narrow the range.",
				len(diff), maxDiffBytes,
			)
		}
		return gomcp.NewToolResultText(diff), nil
	}
}

// diffStatEntry summarizes changes for a single file.
type diffStatEntry struct {
	File    string `json:"file"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// diffStat parses a unified diff into per-file line change summaries.
func diffStat(diff string) []diffStatEntry {
	parts := strings.Split(diff, "diff --git")
	var entries []diffStatEntry
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Extract filename from "a/sites/domain/path b/sites/domain/path"
		firstLine := part
		if before, _, ok := strings.Cut(part, "\n"); ok {
			firstLine = before
		}
		fields := strings.Fields(firstLine)
		file := ""
		if len(fields) >= 2 {
			file = strings.TrimPrefix(fields[1], "b/sites/")
		}

		added, removed := 0, 0
		for line := range strings.SplitSeq(part, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				added++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				removed++
			}
		}
		entries = append(entries, diffStatEntry{File: file, Added: added, Removed: removed})
	}
	return entries
}

// filterDiffToContent strips diff hunks for internal files (lock, config, metadata).
func filterDiffToContent(diff string) string {
	// Split into file-level hunks (each starts with "diff --git")
	parts := strings.Split(diff, "diff --git")
	var kept []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Skip internal metadata files
		if strings.Contains(part, "/.doctrove.lock") ||
			strings.Contains(part, "/_meta/") ||
			strings.Contains(part, "doctrove.yaml") ||
			strings.Contains(part, "doctrove.db") {
			continue
		}
		kept = append(kept, "diff --git"+part)
	}
	return strings.Join(kept, "")
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

func historyHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		limit := IntArg(req, "limit", 20)

		entries, err := e.History(ctx, site, limit)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(entries)
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
		gomcp.WithString("category",
			gomcp.Description("Filter to a specific category (e.g. api-reference, tutorial, guide, spec)"),
		),
		gomcp.WithNumber("limit",
			gomcp.Description("Max files to return (default 100, 0 = all)"),
		),
		gomcp.WithNumber("offset",
			gomcp.Description("Skip first N files for pagination (default 0)"),
		),
	)
}

func listFilesHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		files, err := e.ListFiles(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		// Filter by category if specified
		catFilter := StringArg(req, "category", "")
		if catFilter != "" && !validCategories[catFilter] {
			return gomcp.NewToolResultError(invalidCategoryMsg(catFilter)), nil
		}
		if catFilter != "" {
			var filtered []engine.FileEntry
			for _, f := range files {
				if f.Category == catFilter {
					filtered = append(filtered, f)
				}
			}
			files = filtered
		}

		offset := IntArg(req, "offset", 0)
		limit := IntArg(req, "limit", 100)

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
		if files == nil {
			files = []engine.FileEntry{}
		}

		return JsonResult(files)
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

func removeHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		keepFiles := BoolArg(req, "keep_files", false)
		if err := e.Remove(ctx, site, keepFiles); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(map[string]string{"removed": site})
	}
}

// --- trove_catalog ---

func catalogTool() gomcp.Tool {
	return gomcp.NewTool("trove_catalog",
		gomcp.WithDescription("Get a compact summary of all tracked sites with titles, descriptions, and topic areas — use this to find which site has docs for a topic"),
	)
}

func catalogHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		entries, err := e.Catalog(ctx)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		if entries == nil {
			entries = []engine.CatalogEntry{}
		}
		return JsonResult(entries)
	}
}

// --- trove_stats ---

func statsTool() gomcp.Tool {
	return gomcp.NewTool("trove_stats",
		gomcp.WithDescription("Get workspace statistics: total sites, files, disk usage, and sync freshness"),
	)
}

func statsHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		stats, err := e.Stats(ctx)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return JsonResult(stats)
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

// validCategories is the set of allowed category values.
var validCategories = map[string]bool{
	"api-reference": true, "tutorial": true, "guide": true, "spec": true,
	"changelog": true, "marketing": true, "legal": true, "community": true,
	"context7": true, "index": true, "other": true,
}

func invalidCategoryMsg(category string) string {
	valid := make([]string, 0, len(validCategories))
	for k := range validCategories {
		valid = append(valid, k)
	}
	return fmt.Sprintf("invalid category %q — valid categories: %s", category, strings.Join(valid, ", "))
}

func tagHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		path := StringArg(req, "path", "")
		category := StringArg(req, "category", "")
		if site == "" || path == "" || category == "" {
			return gomcp.NewToolResultError("site, path, and category are required"), nil
		}

		if !validCategories[category] {
			return gomcp.NewToolResultError(invalidCategoryMsg(category)), nil
		}

		if err := e.Tag(ctx, site, path, category); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(map[string]string{
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

func checkHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		result, err := e.Check(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(result)
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

func refreshHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		if site == "" {
			return gomcp.NewToolResultError("site is required"), nil
		}

		result, err := e.Refresh(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return scanResult(result)
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

func outlineHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		path := StringArg(req, "path", "")
		if site == "" || path == "" {
			return gomcp.NewToolResultError("site and path are required"), nil
		}
		maxDepth := IntArg(req, "max_depth", 3)
		maxSections := IntArg(req, "max_sections", 100)

		result, err := e.Outline(ctx, site, path, maxDepth, maxSections)
		if err != nil {
			return gomcp.NewToolResultError(sanitizeError(site, path, err)), nil
		}

		return JsonResult(result)
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

func summarizeHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		path := StringArg(req, "path", "")
		summary := StringArg(req, "summary", "")
		if site == "" || path == "" || summary == "" {
			return gomcp.NewToolResultError("site, path, and summary are required"), nil
		}

		if err := e.Summarize(ctx, site, path, summary); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(map[string]string{
			"site":   site,
			"path":   path,
			"status": "summary saved",
		})
	}
}

// --- trove_stale ---

func staleTool() gomcp.Tool {
	return gomcp.NewTool("trove_stale",
		gomcp.WithDescription("List sites not synced within a threshold (default 7 days). Use this to find sites that may have outdated content and need a refresh."),
		gomcp.WithString("threshold",
			gomcp.Description("Duration threshold (e.g. '7d', '24h', '3d'). Default: 7d"),
		),
	)
}

func staleHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		threshStr := StringArg(req, "threshold", "7d")
		threshold, err := parseDuration(threshStr)
		if err != nil {
			return gomcp.NewToolResultError(fmt.Sprintf("invalid threshold %q: %v", threshStr, err)), nil
		}

		sites, err := e.Stale(ctx, threshold)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		return JsonResult(map[string]any{
			"threshold": threshStr,
			"stale":     sites,
			"count":     len(sites),
		})
	}
}

// parseDuration handles "7d", "24h", "30m" etc. — extends time.ParseDuration with day support.
func parseDuration(s string) (time.Duration, error) {
	if before, ok := strings.CutSuffix(s, "d"); ok {
		days := before
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// --- trove_find ---

func findTool() gomcp.Tool {
	return gomcp.NewTool("trove_find",
		gomcp.WithDescription("Find files by path pattern. Use this when you know roughly what path you want (e.g. '/api/', '/specification/'). Faster than search for path-based lookups."),
		gomcp.WithString("site",
			gomcp.Required(),
			gomcp.Description("The domain (e.g. stripe.com)"),
		),
		gomcp.WithString("pattern",
			gomcp.Required(),
			gomcp.Description("Path substring to match (e.g. '/api/', 'getting-started', '.txt')"),
		),
	)
}

func findHandler(e *engine.Engine) ToolHandler {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		site := StringArg(req, "site", "")
		pattern := StringArg(req, "pattern", "")
		if site == "" || pattern == "" {
			return gomcp.NewToolResultError("site and pattern are required"), nil
		}

		files, err := e.ListFiles(ctx, site)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		patternLower := strings.ToLower(pattern)
		var matches []engine.FileEntry
		for _, f := range files {
			if strings.Contains(strings.ToLower(f.Path), patternLower) {
				matches = append(matches, f)
			}
		}

		return JsonResult(map[string]any{
			"site":    site,
			"pattern": pattern,
			"matches": matches,
			"count":   len(matches),
		})
	}
}

// --- helpers ---

type ToolHandler = func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error)

func JsonResult(v any) (*gomcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return gomcp.NewToolResultText(string(data)), nil
}

// sanitizeError strips filesystem paths from error messages so agents
// only see the logical site/path identifier, not local disk locations.
func sanitizeError(site, path string, err error) string {
	msg := err.Error()
	// Strip any "open /full/path/to/sites/domain/urlpath: " prefix
	if idx := strings.Index(msg, ": open "); idx >= 0 {
		// Find the actual OS error after the filesystem path
		rest := msg[idx+2:]
		if osIdx := strings.LastIndex(rest, ": "); osIdx >= 0 {
			return fmt.Sprintf("file not found: %s%s", site, path)
		}
	}
	// Also catch direct "open /path: no such file" patterns
	if strings.Contains(msg, "no such file or directory") {
		return fmt.Sprintf("file not found: %s%s", site, path)
	}
	return fmt.Sprintf("reading %s%s: %v", site, path, err)
}
