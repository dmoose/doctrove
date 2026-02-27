# MCP Skills

How to use doctrove's MCP tools effectively as an AI agent.

## Setup

Add to your MCP config (Claude Code, Cursor, etc.):

```json
{
  "mcpServers": {
    "doctrove": {
      "command": "doctrove",
      "args": ["mcp"]
    }
  }
}
```

Run `doctrove mcp-config` to generate config with the correct binary path.

## Context7 (Enhanced Discovery)

When a Context7 API key is configured (`context7_api_key` in `doctrove.yaml`), you can resolve bare library names to curated documentation:

```
trove_scan  url="react"
trove_scan  url="stripe-node"
```

This fetches version-aware, community-maintained doc snippets — often higher quality than raw llms.txt content. Results are stored under synthetic domains (e.g. `context7.com~facebook_react`). Get a key at https://context7.com.

## Discovery & Ingestion

### Find out what a site has
```
trove_discover  url="https://stripe.com"
```
Probes for llms.txt, companion files, sitemap, well-known paths. Returns file list with sizes. Does not save anything.

### Add a site and download content
```
trove_scan  url="https://stripe.com"
```
Discovers, tracks, and syncs in one step. Use `content_types="llms-txt"` to only grab the index file. Can be called again on an already-tracked site to widen or change the `content_types` filter without needing to remove and re-add.

### Refresh a tracked site
```
trove_refresh  site="stripe.com"
```
Uses ETag caching — only downloads changed files. Honors any content_types filter from the original scan.

### Check what's stale
```
trove_stale  threshold="7d"
```
Lists sites not synced within the threshold.

## Search & Read

### Search across all docs
```
trove_search  query="authentication"  category="api-reference"  limit=10
```
Returns snippets, categories, and cached summaries. Check summaries before reading full files. Supports FTS5 query syntax. Use `path="/specification/"` to filter by path.

Results include `total_count`, `offset`, `limit`, and `has_more` for pagination. Use `offset` and `limit` to page through large result sets.

### Find files by path
```
trove_find  site="stripe.com"  pattern="/api/"
```
Case-insensitive path substring matching. Faster than search when you know the path.

### Get file structure
```
trove_outline  site="stripe.com"  path="/docs/api.md"  max_depth=2
```
Returns heading tree with section sizes. If a summary exists, it's included — check if it answers your question before reading. When `max_depth` filters out deeper headings, a hint shows how many total sections exist. For files with no headings at all, a hint suggests `max_lines` or search instead.

### Read a specific section
```
trove_read  site="stripe.com"  path="/docs/api.md"  section="Authentication"
```
Case-insensitive heading match. Prefers exact matches and narrower (deeper) headings over broader ones — e.g., `section="Tool"` matches `### Tool` over `# Tools`. Returns content from that heading to the next heading of same/higher level. Use `max_lines=50` to preview.

### Get full content of best match
```
trove_search_full  query="webhook verification"  site="stripe.com"
```
Returns entire file content. Can be very large (100KB+). Prefer outline → section read.

## Browsing

### See what you have
```
trove_catalog
```
Site summaries with titles, topics, and category distribution.

### List files for a site
```
trove_list_files  site="stripe.com"  category="api-reference"  limit=20
```

### Site status
```
trove_status  site="stripe.com"
```
Sync time, file count, category breakdown.

### Workspace stats
```
trove_stats
```
Total sites, files, disk usage, sync freshness.

## Agent Feedback

### Fix a category
```
trove_tag  site="stripe.com"  path="/payments"  category="guide"
```
Persists across re-syncs. Do this when you notice a miscategorized page. Category must be one of: `api-reference`, `tutorial`, `guide`, `spec`, `changelog`, `marketing`, `legal`, `community`, `context7`, `index`, `other`.

### Cache a summary
```
trove_summarize  site="stripe.com"  path="/docs/api.md"  summary="Stripe API reference covering authentication, charges, customers, and webhooks. Read for endpoint signatures and parameters."
```
Future agents see this in search results and outlines.

## History

### Recent changes
```
trove_history  site="stripe.com"  limit=5
```

### Diff between syncs
```
trove_diff  from="HEAD~2"  to="HEAD"
trove_diff  since="2h"                    # all changes in the last 2 hours
trove_diff  since="1d"  stat=true         # last day, compact summary
trove_diff  stat=true                     # compact file-level summary
```
Shows content-only changes (internal metadata filtered out). Use `stat=true` to get a compact summary of changed files with line counts before requesting the full diff. The `since` parameter accepts durations like `30m`, `2h`, `7d`.

## Tips

- **Start with `trove_catalog`** to see what's available before searching
- **Check summaries** in search results before reading full files
- **Use category filters** to narrow results (e.g., `api-reference` for coding, `tutorial` for learning)
- **Summarize after reading** large files — it pays forward to future agents
- **Tag miscategorized pages** — a 2-second fix that improves every future search
