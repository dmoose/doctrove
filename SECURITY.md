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

- **Context7 API key** (`context7_api_key` in config): sent as a Bearer token to `context7.com`. Don't commit to public repos.
- No other credentials are stored or transmitted.

## Network Behavior

doctrove makes outbound HTTP requests to:

1. **Content sources**: websites you explicitly add via `trove_scan` or `doctrove grab`
2. **Context7 API**: `context7.com/api/v2` (only when a valid API key is configured)

No telemetry, no other outbound connections. The MCP server communicates only via stdio.

### Rate Limiting

All outbound requests are rate-limited per host (default: 2 req/sec, burst 5). This is configurable in `doctrove.yaml`. The User-Agent header identifies the tool as `doctrove/0.1`.

### robots.txt

By default, doctrove does **not** check robots.txt because it only fetches content that sites explicitly publish for LLM consumption (llms.txt, etc.). The `--respect-robots` flag enables robots.txt checking for environments that require it.

## MCP Server

The MCP server (`doctrove mcp`) runs as a stdio child process:

- No network listener (stdin/stdout only)
- Read/write access to workspace directory
- Outbound HTTP for discover/sync operations
- No arbitrary command execution

## What NOT to Do

- Don't store API keys or credentials in mirrored content paths
- Don't point doctrove at internal/private documentation URLs (it mirrors to disk as plain files)
- Don't share your workspace directory if it contains a Context7 API key in the config

## Reporting Vulnerabilities

If you discover a security issue, please open a GitHub issue or contact the maintainers directly. We will respond within 3 business days.
