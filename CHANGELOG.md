# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.1.0] - 2026-03-19

Initial public release.

### Added

- **Core mirroring**: discover, download, and track LLM-targeted documentation from websites
- **20 MCP tools** for agent integration: discover, scan, search, read, outline, catalog, status, stats, history, diff, tag, summarize, refresh, check, stale, find, list, list_files, remove, search_full
- **CLI commands** mirroring all MCP tools plus grab (discover+track+sync), init, sync, mcp, mcp-config
- **Content discovery**: well-known paths (llms.txt, ai.txt), companion file parsing, sitemap probing, seed path detection, platform detection (MkDocs, Docusaurus, Sphinx, GitBook)
- **Context7 integration**: resolve bare library names to curated documentation via Context7 API with automatic retry on transient errors (202, 429, 5xx)
- **HTML-to-markdown conversion** for sites serving HTML at LLM content URLs, with JS-heavy SPA detection and MDX/JSX artifact cleanup
- **SQLite FTS5 full-text search** with path boosting, category filtering, pagination (offset/limit/has_more), and total count
- **Git-based change tracking**: every sync is committed, with history and diff tools including time-based ranges (`since` parameter)
- **Page categorization**: 11 semantic categories (api-reference, tutorial, guide, spec, changelog, marketing, legal, community, context7, index, other) assigned by path heuristics and body analysis
- **Agent feedback loop**: trove_tag for persistent category overrides, trove_summarize for cached summaries searchable via FTS
- **ETag caching**: conditional fetch on refresh to skip unchanged files
- **Path collision resolution**: automatic file-to-directory promotion when parent/child URL paths conflict (e.g. `/deploy` and `/deploy/getting_started`)
- **Rate limiting**: per-host configurable rate limit with burst capacity
- **Event relay integration**: optional structured event emission for observability
- **Functional options API**: all engine components injectable for library usage
- **10 swappable interfaces**: HTTPFetcher, Syncer, ContentDiscoverer, VersionStore, Indexer, Categorizer, Processor, EventEmitter, and more

### Documentation

- README with install, quick start, all commands, MCP setup, categories, configuration
- AGENT.md agent-focused guide
- DESIGN.md architecture and package layout
- Skills guides for MCP and CLI usage
- Context7 integration docs with Upstash terms notice
