# MCP Skills

MCP tool reference for doctrove.

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

Results are stored under synthetic domains (e.g. `context7.com~facebook_react`). Get a key at https://context7.com.

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
Uses ETag caching; only downloads changed files. Honors the content_types filter from the original scan.

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
Returns snippets, categories, and cached summaries. Supports FTS5 query syntax. Use `path="/specification/"` to filter by path. Paginated: response includes `total_count`, `offset`, `limit`, `has_more`.

### Find files by path
```
trove_find  site="stripe.com"  pattern="/api/"
```
Case-insensitive path substring matching. Faster than search when you know the path.

### Get file structure
```
trove_outline  site="stripe.com"  path="/docs/api.md"  max_depth=2
```
Returns heading tree with section sizes. Includes cached summary if one exists. When `max_depth` filters sections, a hint shows the total count.

### Read a specific section
```
trove_read  site="stripe.com"  path="/docs/api.md"  section="Authentication"
```
Case-insensitive heading match. Prefers exact matches and deeper headings (e.g. `section="Tool"` matches `### Tool` over `# Tools`). Returns content from that heading to the next same/higher-level heading. Use `max_lines=50` to preview.

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
Persists across re-syncs. Valid categories: `api-reference`, `tutorial`, `guide`, `spec`, `changelog`, `marketing`, `legal`, `community`, `context7`, `index`, `other`.

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
Content-only changes (metadata filtered out). `stat=true` gives a compact file-level summary. `since` accepts durations: `30m`, `2h`, `7d`.
