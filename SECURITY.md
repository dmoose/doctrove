# Security

## Threat Model

doctrove is a local development tool that mirrors publicly available documentation to a local workspace. The threat model assumes:

- The operator controls the machine and config file
- Content sources are public websites publishing LLM-targeted documentation
- The MCP server runs locally as a child process of the agent host (Claude Code, Cursor, etc.)
- The workspace directory contains only mirrored documentation, not secrets

## Data Handling

| Data | Storage | Risk |
|------|---------|------|
| Mirrored content | Plain files under `~/.config/doctrove/sites/` | Public documentation, no secrets. Git-tracked for change history. |
| SQLite index | `doctrove.db` in workspace | FTS5 index of mirrored content. Same data as the files. |
| Config file | `doctrove.yaml` in workspace | May contain a Context7 API key. Protect with standard file permissions. |
| Git history | `.git/` in workspace | Records when content was synced. No credentials stored. |

## API Keys

- **Context7 API key** (`context7_api_key` in config) — sent as a Bearer token to `context7.com`. Treat like any API credential: don't commit to public repos, don't share in logs.
- No other credentials are stored or transmitted.

## Network Behavior

doctrove makes outbound HTTP requests to:

1. **Content sources** — websites you explicitly add via `trove_scan` or `doctrove grab`
2. **Context7 API** — `context7.com/api/v2` (only when a valid API key is configured)

It does **not** phone home, send telemetry, or contact any other services. The MCP server communicates only via stdio (no network listener).

### Rate Limiting

All outbound requests are rate-limited per host (default: 2 req/sec, burst 5). This is configurable in `doctrove.yaml`. The User-Agent header identifies the tool as `doctrove/0.1`.

### robots.txt

By default, doctrove does **not** check robots.txt because it only fetches content that sites explicitly publish for LLM consumption (llms.txt, etc.). The `--respect-robots` flag enables robots.txt checking for environments that require it.

## MCP Server

The MCP server (`doctrove mcp`) runs as a stdio-based child process. It:

- Has no network listener — communicates only via stdin/stdout
- Has read/write access to the workspace directory
- Can make outbound HTTP requests (for discover/sync operations)
- Cannot execute arbitrary commands

## What NOT to Do

- Do not store API keys or credentials in mirrored content paths
- Do not point doctrove at internal/private documentation URLs — it mirrors content to disk as plain files
- Do not share your workspace directory if it contains a Context7 API key in the config

## Reporting Vulnerabilities

If you discover a security issue, please open a GitHub issue or contact the maintainers directly. We will respond within 3 business days.
