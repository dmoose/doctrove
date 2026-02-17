# llmshadow

A central store for LLM-targeted documentation. Discovers, mirrors, and indexes content from sites that publish `llms.txt`, companion `.html.md` files, and related formats. Designed for developers who want local, searchable access to LLM-friendly docs — and for AI agents that need to find and read them.

## Install

```bash
make install                   # builds and installs to /usr/local/bin
llmshadow mcp-config           # shows config to add to your agent
```

Workspace defaults to `~/.config/llmshadow`. Override with `--dir` or `LLMSHADOW_DIR`.

## Quick Start

```bash
# Discover what a site has
llmshadow discover https://stripe.com

# Grab it (init + sync in one step)
llmshadow grab https://supabase.com

# Search across all mirrored content
llmshadow search "authentication"

# See what you have
llmshadow catalog
llmshadow stats
```

## Commands

| Command | Description |
|---|---|
| `discover <url>` | Probe a URL for LLM content without tracking |
| `grab <url>` | Discover, track, and sync in one step |
| `init <url>` | Add a site to track |
| `sync [site\|--all]` | Download/update content |
| `search <query>` | Full-text search with `--site`, `--type`, `--full` |
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
llmshadow mcp-config
```

Add the `mcpServers` entry to your agent's config file:

| Agent | Config File |
|---|---|
| Claude Code | `~/.claude/claude_code_config.json` |
| Cursor | `.cursor/mcp.json` (project root) |

Example config:

```json
{
  "mcpServers": {
    "llmshadow": {
      "command": "/usr/local/bin/llmshadow",
      "args": ["mcp", "--dir", "/Users/you/.config/llmshadow"]
    }
  }
}
```

### Tools (13)

| Tool | Description |
|---|---|
| `shadow_discover` | Probe a URL for LLM content |
| `shadow_scan` | Add and sync a site in one call |
| `shadow_search` | Full-text search (returns suggestion on empty results) |
| `shadow_search_full` | Search and return full content of best match |
| `shadow_read` | Read a file with freshness metadata |
| `shadow_list` | List tracked sites |
| `shadow_list_files` | Enumerate files in a site |
| `shadow_catalog` | Site summaries with topics |
| `shadow_stats` | Workspace statistics |
| `shadow_status` | Sync status for a site |
| `shadow_history` | Git change history |
| `shadow_diff` | Content changes between refs |
| `shadow_remove` | Stop tracking a site |

## Content Discovery

llmshadow probes multiple sources for LLM-targeted content:

- **Well-known paths:** `/llms.txt`, `/llms-full.txt`, `/llms-ctx.txt`, `/llms-ctx-full.txt`, `/ai.txt`
- **Companion files:** URLs referenced in llms.txt (markdown links followed permissively)
- **Sitemap:** Checks `sitemap.xml` for paths containing `/llms/` or ending in `.md`/`.txt`
- **.well-known:** `tdmrep.json`, `agent.json`, `agents.json`
- **HTML conversion:** Sites serving HTML at content URLs (Next.js, SPAs) are converted to markdown

## Configuration

`llmshadow.yaml` in the workspace root:

```yaml
settings:
  rate_limit: 2            # req/sec per host
  rate_burst: 5            # burst capacity
  timeout: 30s             # HTTP timeout
  max_probes: 100          # companion probes per llms.txt
  user_agent: "llmshadow/0.1"
  events_url: http://localhost:6060/events  # optional eventrelay integration

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
--dir string         workspace directory (default ".")
--json               output as JSON
--respect-robots     respect robots.txt AI crawler directives (off by default)
```

## Storage

Content is stored as plain files under `sites/<domain>/`, tracked by git for change history, with a SQLite FTS5 index for search. The git repo and index are the workspace — share it by cloning.

## Event Relay Integration

When `events_url` is configured, llmshadow emits structured events to an [eventrelay](../eventrelay) server for real-time observability. Events follow the full eventrelay schema:

```json
{
  "source": "llmshadow",
  "channel": "mcp",
  "action": "shadow_search",
  "level": "info",
  "agent_id": "myproject:00a3f1",
  "duration_ms": 42,
  "data": {"query": "authentication", "site": "stripe.com"},
  "ts": "2026-03-18T12:00:00Z"
}
```

| Field | Description |
|---|---|
| `source` | Always `llmshadow` |
| `channel` | `mcp` for MCP tool calls, `sync` for engine operations (init, sync, discover, remove) |
| `action` | Tool or operation name (e.g. `shadow_search`, `sync`, `init`) |
| `level` | `info` normally, `error` on failure, `warn` on partial errors |
| `agent_id` | Auto-derived from working directory + PID (e.g. `myproject:00a3f1`) |
| `duration_ms` | Operation wall time (top-level, displayed inline in the dashboard) |
| `data` | Tool arguments (MCP) or operation details (engine) |
