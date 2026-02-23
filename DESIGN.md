# doctrove Design Document

A Go tool that discovers, downloads, and maintains local mirrors of websites' LLM-targeted content (llms.txt, companion files, etc.) with git-based change tracking, full-text search, and an MCP interface for agent access.

## Architecture

Three layers — interfaces, core engine, storage — so CLI, MCP, and Go library consumers all share the same logic.

```
┌─────────────────────────────────────────────────┐
│  Interfaces                                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │   CLI   │  │   MCP   │  │  Go Library API  │ │
│  └────┬────┘  └────┬────┘  └────────┬────────┘ │
│       └─────────┬──┴───────────────┬┘           │
├─────────────────┼──────────────────┼────────────┤
│  Core Engine    │                  │             │
│  ┌──────────────▼──────────────────▼──────────┐ │
│  │  engine.Engine                              │ │
│  │  ┌────────────┐ ┌──────────┐ ┌───────────┐ │ │
│  │  │ Discovery  │ │  Mirror  │ │   Index   │ │ │
│  │  └────────────┘ └──────────┘ └───────────┘ │ │
│  │  ┌────────────┐ ┌──────────┐ ┌───────────┐ │ │
│  │  │  Content   │ │   Git    │ │  Config   │ │ │
│  │  └────────────┘ └──────────┘ └───────────┘ │ │
│  └────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────┤
│  Storage                                         │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐ │
│  │Filesystem│  │   Git    │  │ SQLite FTS5   │ │
│  └──────────┘  └──────────┘  └───────────────┘ │
└─────────────────────────────────────────────────┘
```

## Package Layout

```
doctrove/
├── cmd/doctrove/
│   └── main.go                    # Entry point
├── internal/
│   ├── engine/
│   │   ├── engine.go              # Engine struct, New(), Close()
│   │   ├── sync.go                # Sync, SyncWithContentTypes, SyncAll, Refresh
│   │   ├── read.go                # Outline, ReadSection (delegates to ContentProcessor)
│   │   ├── search.go              # Search, SearchFull, RebuildIndex
│   │   ├── manage.go              # Init, Discover, Status, List, Check, History, Diff,
│   │   │                          #   ListFiles, Remove, Tag, Summarize
│   │   ├── catalog.go             # Catalog (llms.txt topic extraction)
│   │   └── stats.go               # Stats, Stale, humanAge/humanSize
│   ├── content/
│   │   ├── content.go             # Processor interface (Outline, ReadSection)
│   │   ├── markdown.go            # MarkdownProcessor (default)
│   │   └── summarizer.go          # Summarizer interface + NoOpSummarizer
│   ├── discovery/
│   │   ├── discovery.go           # Provider interface + Discoverer orchestrator
│   │   ├── wellknown.go           # /llms.txt, /llms-full.txt, /ai.txt probing
│   │   ├── companion.go           # Companion file parsing from llms.txt links
│   │   ├── context7.go            # Context7 API provider (optional)
│   │   └── sitemap.go             # Sitemap-based discovery
│   ├── mirror/
│   │   ├── mirror.go              # Download, store, include/exclude filtering
│   │   └── rewriter.go            # URL → local path link rewriting
│   ├── store/
│   │   ├── store.go               # Filesystem layout abstraction
│   │   ├── git.go                 # Git init (with seed commit), commit, log, diff
│   │   ├── index.go               # SQLite FTS5 search index
│   │   ├── indexer.go             # Indexer interface
│   │   └── categorize.go          # Categorizer interface + RuleCategorizer
│   ├── config/
│   │   └── config.go              # YAML config loading, defaults
│   ├── fetcher/
│   │   ├── fetcher.go             # HTTP client: rate limiting, ETags, conditional requests
│   │   └── convert.go             # HTML-to-markdown conversion
│   ├── events/
│   │   └── emitter.go             # Structured event emission to eventrelay
│   ├── robots/
│   │   └── robots.go              # robots.txt compliance checking
│   └── lockfile/
│       └── lockfile.go            # Workspace concurrency lock
├── cli/
│   ├── root.go                    # Cobra root command, --dir/--json/--respect-robots
│   ├── grab.go                    # doctrove grab <url> (init + sync)
│   └── ...                        # One file per command (19 commands)
├── mcp/
│   ├── server.go                  # MCP server setup, tool registration, tracing
│   └── tools.go                   # 18 MCP tool handlers → engine calls
├── go.mod
└── go.sum
```

## Interfaces

The system uses interfaces at key extension points so implementations can be swapped:

### Provider (discovery)
```go
type Provider interface {
    Name() string
    CanHandle(input string) bool
    Discover(ctx context.Context, input string) (*Result, error)
}
```
Implementations: `SiteProvider` (default, handles URLs), `Context7Provider` (bare library names).

### Indexer (search/storage)
```go
type Indexer interface {
    IndexFile(domain, path, contentType, body string, category ...string) error
    Search(query string, opts SearchOpts) ([]SearchHit, error)
    DeleteSite(domain string) error
    Rebuild(store *Store) error
    GetCacheHeaders(domain, path string) (etag, lastModified string, err error)
    UpdateCacheHeaders(domain, path, etag, lastModified string) error
    GetCategory(domain, path string) (string, error)
    SetCategory(domain, path, category string) error
    GetSummary(domain, path string) (summary, summaryAt string, err error)
    SetSummary(domain, path, summary string) error
    CategoryCounts(domain string) (map[string]int, error)
    Close() error
}
```
Default implementation: SQLite FTS5 with porter tokenization.

### Processor (content parsing)
```go
type Processor interface {
    Name() string
    CanProcess(path, contentType string) bool
    Outline(content string, maxDepth, maxSections int) OutlineResult
    ReadSection(content, sectionName string) (string, error)
}
```
Default implementation: `MarkdownProcessor` (ATX heading parser). Future: reStructuredText, AsciiDoc.

### Categorizer (page classification)
```go
type Categorizer interface {
    Categorize(domain, path, contentType, body string) string
}
```
Default implementation: `RuleCategorizer` (path patterns + body heuristics). Future: ML-based, LLM-based.

### Summarizer (content summarization)
```go
type Summarizer interface {
    Summarize(ctx context.Context, domain, path, content string) (string, error)
}
```
Default implementation: `NoOpSummarizer` (relies on agent-submitted summaries). Future: LLM-based auto-summarization, extractive methods.

## Core Engine

```go
type Engine struct {
    Config      *config.Config
    Store       *store.Store
    Git         *store.GitStore
    Index       store.Indexer
    Discovery   *discovery.Discoverer
    Mirror      *mirror.Mirror
    Fetcher     *fetcher.Fetcher
    Events      *events.Emitter
    Processors  []content.Processor
    Categorizer store.Categorizer
    RootDir     string
}
```

All methods return structured data — CLI formats for humans, MCP returns JSON. Key methods:

| Group | Methods |
|---|---|
| Lifecycle | `Init`, `Remove`, `Discover` |
| Sync | `Sync`, `SyncWithContentTypes`, `SyncAll`, `Refresh` |
| Read | `Outline`, `ReadSection` |
| Search | `Search`, `SearchFull`, `RebuildIndex` |
| Query | `Status`, `List`, `Check`, `ListFiles`, `Catalog`, `Stats` |
| History | `History`, `Diff` |
| Feedback | `Tag`, `Summarize` |

## Content Discovery

Discovery uses the Provider pattern — multiple providers are tried in order:

1. **Well-known paths:** `/llms.txt`, `/llms-full.txt`, `/llms-ctx.txt`, `/llms-ctx-full.txt`, `/ai.txt`, `/.well-known/tdmrep.json`, `/.well-known/agent.json`
2. **Companion files:** Markdown links and bare URLs parsed from llms.txt (capped at `max_probes` per site)
3. **Sitemap:** `sitemap.xml` paths containing `/llms/` or ending in `.md`/`.txt`
4. **Context7 API:** Bare library names resolved to curated docs (optional, requires API key)
5. **HTML conversion:** Sites serving HTML at content URLs are auto-converted to markdown

## Storage

### Filesystem Layout
```
<workspace>/
├── .git/                        # Git repo for change tracking
├── doctrove.yaml                # Configuration
├── doctrove.db                  # SQLite FTS5 index (gitignored)
└── sites/
    └── <domain>/
        ├── llms.txt
        ├── llms-full.txt
        ├── docs/*.md            # Companion files
        └── _meta/
            ├── discovered.json
            └── links.json
```

### Git Integration
- Fresh workspaces get a seed commit (HEAD is valid from the start)
- Each sync auto-commits changes
- Git failures are non-fatal — content is downloaded and indexed regardless
- Partial `.git` directories are auto-recovered via `ensureHead()`

### Search Index
SQLite FTS5 with porter + unicode61 tokenization. Concurrent access from CLI and MCP via WAL mode. Search results include:
- FTS5 snippets with `**bold**` match highlighting
- Cached agent-submitted summaries
- Categories for filtering

## Page Categories

| Category | Assigned by |
|---|---|
| `api-reference` | Path `/api/`, `/reference/`, or ≥3 code blocks |
| `tutorial` | Path `/tutorials/`, `/getting-started/`, `/quickstart` |
| `guide` | Path `/guides/`, `/learn/`, `/how-to/` |
| `spec` | Path `/specification/`, `/schema` |
| `changelog` | Path `/changelog`, `/release-notes` |
| `marketing` | Path `/pricing`, `/use-cases/`, or link-heavy pages |
| `legal` | Path `/privacy`, `/legal/`, `/terms` |
| `community` | Path `/community/`, `/seps/`, `/contributing` |
| `context7` | Content from Context7 API |
| `index` | llms.txt, llms-full.txt, ai.txt family |
| `other` | Unclassified, well-known metadata |

Categories are auto-assigned by `RuleCategorizer` (path patterns, then body heuristics). User overrides via `Tag()` persist across re-syncs.

## MCP Tools (18)

| Tool | Engine Method |
|---|---|
| `trove_discover` | `Discover()` |
| `trove_scan` | `Init()` + `SyncWithContentTypes()` |
| `trove_search` | `Search()` |
| `trove_search_full` | `SearchFull()` |
| `trove_list` | `List()` |
| `trove_read` | `ReadSection()` |
| `trove_status` | `Status()` |
| `trove_diff` | `Diff()` |
| `trove_history` | `History()` |
| `trove_list_files` | `ListFiles()` |
| `trove_remove` | `Remove()` |
| `trove_catalog` | `Catalog()` |
| `trove_stats` | `Stats()` |
| `trove_tag` | `Tag()` |
| `trove_refresh` | `Refresh()` |
| `trove_check` | `Check()` |
| `trove_outline` | `Outline()` (with `max_depth`, `max_sections` caps) |
| `trove_summarize` | `Summarize()` |

All tool calls are traced via the event emitter with wall time and agent_id.

## Context-Efficient Workflow

Tools are designed for hierarchical drill-down to minimize LLM context usage:

```
trove_catalog          → which site has docs on my topic?
trove_search           → which files are relevant? (check summaries first)
trove_outline          → what sections? (capped at depth 3, 100 sections)
trove_read section=X   → read just the section I need
trove_summarize        → cache summary so next agent skips re-reading
```

## Fetcher

All HTTP goes through `fetcher.Fetcher`:
- Per-domain rate limiting (`golang.org/x/time/rate`, default 2/sec burst 5)
- ETag / Last-Modified conditional requests for efficient re-syncs
- HTML detection and auto-conversion to markdown
- Configurable user-agent, timeout

## Dependencies

- **cobra** — CLI framework
- **go-git/go-git** — git operations (no shell exec)
- **modernc.org/sqlite** — SQLite FTS5 (pure Go, CGo-free)
- **mark3labs/mcp-go** — MCP server (stdio transport)
- **gopkg.in/yaml.v3** — config
- **golang.org/x/time/rate** — rate limiting
- **JohannesKaufmann/html-to-markdown** — HTML conversion
