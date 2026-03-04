# Agent Guide

Local documentation store for AI coding agents. Mirrors LLM-targeted content (llms.txt, companions) from websites with full-text search, git change tracking, and category filtering.

## MCP vs CLI

doctrove exposes the same capabilities through two interfaces:

- **MCP tools** (20 tools): primary interface for agents in Claude Code, Cursor, etc. See `skills/mcp.md`.
- **CLI commands**: shell workflows, scripting, debugging. See `skills/cli.md`.

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

- **`trove_tag`**: fix a miscategorized page. Persists across re-syncs.
- **`trove_summarize`**: write a 2-5 sentence summary after reading a large file. Visible in search results and outlines for future agents.

Both persist across re-syncs.

## Categories

Every mirrored file has a category for task-appropriate filtering:

| Category | Use when |
|---|---|
| `api-reference` | Looking up function signatures, endpoints, parameters |
| `tutorial` | Step-by-step walkthroughs |
| `guide` | Concepts, architecture, best practices |
| `spec` | Checking protocol or schema definitions, SEPs/proposals |
| `changelog` | Version history |
| `index` | Site overview (llms.txt) |
| `community` | Contributing, governance |
| `other` | Everything else |

Use the `category` parameter on `trove_search` and `trove_list_files` to filter.

## When to add a site

If you're working on a project and need documentation for a library or service:

1. Check if it's already tracked: `trove_list`
2. If not, discover what's available: `trove_discover <url>`
3. If it has content, add it: `trove_scan <url>`

Sites with llms.txt: Stripe, Supabase, Vercel, Deno, Turso, and others. Sites without llms.txt may still have content discoverable via sitemap or seed probing.

With a Context7 API key (`context7_api_key` in `doctrove.yaml`), you can also add bare library names like `react` or `stripe-node`.

## Keeping content fresh

- **`trove_refresh`**: re-syncs using ETag caching (skips unchanged files)
- **`trove_stale`**: lists sites not synced within a threshold (default 7 days)
