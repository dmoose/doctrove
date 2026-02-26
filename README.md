# doctrove

A local documentation store for AI coding agents.

Many websites now publish LLM-friendly documentation via [`llms.txt`](https://llmstxt.org/) — a proposed standard (like `robots.txt` for crawlers) where sites provide markdown summaries, companion files, and structured content specifically for language models. doctrove discovers, mirrors, and indexes this content so agents have fast, searchable, offline access to documentation — without burning tokens on web fetches or context windows on full pages.

## Install

```bash
make install                   # builds and installs to $GOBIN
make init-workspace            # creates ~/.config/doctrove with default config
doctrove mcp-config            # shows config to add to your agent
```

Workspace defaults to `~/.config/doctrove`. Override with `--dir` or `DOCTROVE_DIR`.

## Quick Start

```bash
# Discover what a site has
doctrove discover https://stripe.com

# Grab it (init + sync in one step)
doctrove grab https://supabase.com

# Search across all mirrored content
doctrove search "authentication"

# Search only API docs
doctrove search --category api-reference "webhooks"

# Refresh to pick up changes (uses ETag caching)
doctrove refresh supabase.com

# See what you have
doctrove catalog
doctrove stats
```

## Commands

| Command | Description |
|---|---|
| `discover <url>` | Probe a URL for LLM content without tracking |
| `grab <url>` | Discover, track, and sync in one step |
| `init <url>` | Add a site to track |
| `sync [site\|--all]` | Download/update content |
| `refresh [site\|--all]` | Re-sync tracked sites, skipping unchanged files via ETag caching |
| `search <query>` | Full-text search with `--site`, `--type`, `--category`, `--full` |
| `tag <site> <path> <cat>` | Override the category for a mirrored file |
| `catalog` | Show site summaries with topics (from llms.txt structure) |
| `stats` | Disk usage, file counts, sync freshness per site |
| `stale` | Show sites not synced within `--threshold` (default 7d) |
| `list` | List all tracked sites |
| `status [site]` | Show sync status and file counts |
| `check <site>` | Dry-run: show available content without downloading |
| `history [site]` | Git-based change history with `--since` |
| `diff [from] [to]` | Show content changes between syncs |
| `remove <site>` | Stop tracking (with `--keep-files` option) |
| `mcp` | Start MCP server (stdio transport) |

All commands support `--json` for machine-readable output.

## MCP Server

Generate your config snippet:

```bash
doctrove mcp-config
```

Add the `mcpServers` entry to the appropriate config file:

| Agent | Config File |
|---|---|
| Claude Code (user scope) | `~/.claude.json` |
| Claude Code (project scope) | `.mcp.json` (project root) |
| Cursor | `.cursor/mcp.json` (project root) |

Example config:

```json
{
  "mcpServers": {
    "doctrove": {
      "command": "/usr/local/bin/doctrove",
      "args": ["mcp", "--dir", "/Users/you/.config/doctrove"]
    }
  }
}
```

### Tools (20)

| Tool | Description |
|---|---|
| `trove_discover` | Probe a URL for LLM content |
| `trove_scan` | Add and sync a site (`content_types` param to filter; persisted for refresh) |
| `trove_refresh` | Re-sync a tracked site, using ETag caching (honours content_types filter) |
| `trove_check` | Dry-run: show available content without downloading |
| `trove_search` | Full-text search with `category`, `path` filters; path-boosted ranking; summaries included |
| `trove_search_full` | Search and return full content of best match (large — prefer outline+section read) |
| `trove_outline` | Get heading structure with `max_depth` (default 3) and `max_sections` (default 100) caps |
| `trove_read` | Read a file or specific section by heading match (`section` param) |
| `trove_summarize` | Store an agent-written summary for a file (visible in search results and outlines) |
| `trove_tag` | Override category for a file (persists across re-syncs) |
| `trove_list` | List tracked sites |
| `trove_list_files` | Enumerate files with path, size, content type, and category (paginated, `category` filter) |
| `trove_catalog` | Site summaries with topics |
| `trove_stats` | Workspace statistics |
| `trove_status` | Sync status, category breakdown, and staleness for a site |
| `trove_history` | Git change history |
| `trove_diff` | Content changes between refs |
| `trove_stale` | List sites not synced within a threshold (default 7d) |
| `trove_find` | Find files by path pattern (faster than search for path lookups) |
| `trove_remove` | Stop tracking a site |

### Context-Efficient Workflow

The tools are designed for hierarchical drill-down to minimize context usage:

```
trove_catalog          → which site has docs on my topic?
trove_search           → which files are relevant? (check summaries first)
trove_outline          → what sections does this file have? (+ summary if cached)
trove_read section=X   → read just the section I need
trove_summarize        → cache a summary so the next agent doesn't re-read
```

**Agent feedback loop:** `trove_tag` and `trove_summarize` are persistent — they survive re-syncs and help all future agents. If you read a large file, summarize it. If a category is wrong, fix it. These small investments compound across sessions.

## Content Discovery

doctrove probes multiple sources for LLM-targeted content:

- **Well-known paths:** `/llms.txt`, `/llms-full.txt`, `/llms-ctx.txt`, `/llms-ctx-full.txt`, `/ai.txt`
- **Companion files:** URLs referenced in llms.txt (markdown links followed permissively)
- **Sitemap:** Checks `sitemap.xml` for paths containing `/llms/` or ending in `.md`/`.txt`
- **.well-known:** `tdmrep.json`, `agent.json`, `agents.json`
- **Context7:** Bare library names (e.g. `react`, `stripe-node`) resolved via Context7 API when `context7_api_key` is configured
- **HTML conversion:** Sites serving HTML at content URLs (Next.js, SPAs) are converted to markdown

## Page Categories

Every indexed file is assigned a semantic category for task-appropriate filtering:

| Category | Examples |
|---|---|
| `api-reference` | `/api/`, `/reference/`, code-heavy pages |
| `tutorial` | `/tutorials/`, `/getting-started/`, `/quickstart` |
| `guide` | `/guides/`, `/learn/`, `/how-to/` |
| `spec` | `/specification/`, `/schema` |
| `changelog` | `/changelog`, `/release-notes` |
| `marketing` | `/pricing`, `/use-cases/`, `/customers`, link-heavy pages |
| `legal` | `/privacy`, `/legal/`, `/terms` |
| `community` | `/community/`, `/seps/`, `/contributing` |
| `context7` | Content fetched via Context7 API |
| `index` | llms.txt, llms-full.txt, ai.txt — site index files |
| `other` | Unclassified companions, well-known metadata |

Categories are assigned by path heuristics (fast) with body analysis as fallback. Agents can override with `trove_tag` / `doctrove tag`.

```bash
# Search only API docs
doctrove search --category api-reference "hooks"

# Fix a misclassified page
doctrove tag stripe.com /payments marketing
```

## ETag Caching

Re-syncs use HTTP conditional requests (`If-None-Match`, `If-Modified-Since`) to skip unchanged files. Cache headers are stored per-file in the index. Use `refresh` to take advantage of this:

```bash
doctrove refresh modelcontextprotocol.io   # only downloads changed files
```

## Configuration

`doctrove.yaml` in the workspace root:

```yaml
settings:
  rate_limit: 2            # req/sec per host
  rate_burst: 5            # burst capacity
  timeout: 30s             # HTTP timeout
  max_probes: 100          # companion probes per llms.txt
  user_agent: "doctrove/0.1"
  events_url: http://localhost:6060/events    # optional eventrelay integration
  context7_api_key: ctx7sk-...                # optional Context7 API key

sites:
  stripe.com:
    url: https://stripe.com
    include:
      - "/llms*"
      - "/docs/**/*.md"
    exclude:
      - "/internal/**"
```

## Global Flags

```
--dir string         workspace directory (default ~/.config/doctrove)
--json               output as JSON
--respect-robots     respect robots.txt AI crawler directives (off by default)
```

## Storage

Content is stored as plain files under `sites/<domain>/`, tracked by git for change history, with a SQLite FTS5 index for search. The git repo and index are the workspace — share it by cloning.

## Event Relay Integration

When `events_url` is configured, doctrove emits structured events to an [eventrelay](../eventrelay) server for real-time observability. Events follow the full eventrelay schema:

```json
{
  "source": "doctrove",
  "channel": "mcp",
  "action": "trove_search",
  "level": "info",
  "agent_id": "myproject:00a3f1",
  "duration_ms": 42,
  "data": {"query": "authentication", "site": "stripe.com"},
  "ts": "2026-03-18T12:00:00Z"
}
```

| Field | Description |
|---|---|
| `source` | Always `doctrove` |
| `channel` | `mcp` for MCP tool calls, `sync` for engine operations (init, sync, discover, remove) |
| `action` | Tool or operation name (e.g. `trove_search`, `sync`, `init`) |
| `level` | `info` normally, `error` on failure, `warn` on partial errors |
| `agent_id` | Auto-derived from working directory + PID (e.g. `myproject:00a3f1`) |
| `duration_ms` | Operation wall time (top-level, displayed inline in the dashboard) |
| `data` | Tool arguments (MCP) or operation details (engine) |
