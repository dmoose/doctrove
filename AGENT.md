# Agent Guide

doctrove is a documentation store designed for AI coding agents. This file explains how to use it effectively — whether you're accessing it via MCP tools or the CLI.

## What doctrove does

doctrove mirrors LLM-targeted documentation from websites to a local store with full-text search, git change tracking, and category filtering. You can discover what documentation a site publishes, download it, search across all mirrored content, and drill into specific sections — all without web fetching.

## MCP vs CLI

doctrove exposes the same capabilities through two interfaces:

- **MCP tools** (20 tools) — for agents running inside Claude Code, Cursor, or other MCP-capable hosts. This is the primary interface. See `skills/mcp.md`.
- **CLI commands** — for shell-based workflows, scripting, and debugging. See `skills/cli.md`.

Both operate on the same workspace (`~/.config/doctrove` by default).

## Core workflow

The tools are designed for **hierarchical drill-down** to minimize context usage:

```
1. trove_catalog          → which site has docs on my topic?
2. trove_search           → which files are relevant? (check summaries first)
3. trove_outline          → what sections does this file have?
4. trove_read section=X   → read just the section I need
5. trove_summarize        → cache a summary so the next agent skips re-reading
```

**Do not** use `trove_search_full` or read entire files unless you genuinely need all the content. The outline → section read pattern saves significant context.

## Agent feedback loop

Two tools let you improve the trove for future agents:

- **`trove_tag`** — Fix a miscategorized page (e.g., a tutorial tagged as `api-reference`). Persists across re-syncs.
- **`trove_summarize`** — After reading a large file, write a 2-5 sentence summary. Future agents will see it in search results and outlines, letting them decide whether to read the full content.

These are persistent — they survive re-syncs and help every agent that uses this workspace. Small investments that compound.

## Categories

Every mirrored file has a category for task-appropriate filtering:

| Category | Use when |
|---|---|
| `api-reference` | Looking up function signatures, endpoints, parameters |
| `tutorial` | Learning how to do something step by step |
| `guide` | Understanding concepts, architecture, best practices |
| `spec` | Checking protocol or schema definitions |
| `changelog` | Finding what changed between versions |
| `index` | Getting an overview of what a site covers (llms.txt) |
| `community` | Contributing guidelines, governance, proposals |
| `other` | Everything else |

Use the `category` parameter on `trove_search` and `trove_list_files` to filter.

## When to add a site

If you're working on a project and need documentation for a library or service:

1. Check if it's already tracked: `trove_list`
2. If not, discover what's available: `trove_discover <url>`
3. If it has content, add it: `trove_scan <url>`

Common sites with good llms.txt coverage: Stripe, Supabase, Cloudflare, Vercel, modelcontextprotocol.io, and many more. Sites without llms.txt may still have useful content discovered via sitemap or seed probing.

## Keeping content fresh

- **`trove_refresh`** — Re-syncs a site using ETag caching (only downloads changed files)
- **`trove_stale`** — Lists sites not synced within a threshold (default 7 days)

Check stale sites periodically if you're relying on the documentation being current.
