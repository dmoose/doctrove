# doctrove Design Document

A Go tool that discovers, downloads, and maintains local mirrors of websites' LLM-targeted content (llms.txt, companion files, etc.) with git-based change tracking, full-text search, and an MCP interface for agent access.

Also usable as a Go library — all public packages can be imported and components swapped via functional options.

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

All packages are public (importable) except `internal/robots` and `internal/lockfile`.

```
doctrove/
├── cmd/doctrove/
│   └── main.go                    # Entry point
├── engine/
│   ├── engine.go                  # Engine struct, New() with functional options
│   ├── sync.go                    # Sync, SyncWithContentTypes, SyncAll, Refresh
│   ├── read.go                    # Outline, ReadSection
│   ├── search.go                  # Search, SearchFull, RebuildIndex
│   ├── manage.go                  # Init, Discover, Status, List, Check, History, Diff,
│   │                              #   ListFiles, Remove, Tag, Summarize
│   ├── catalog.go                 # Catalog (llms.txt topic + category extraction)
│   └── stats.go                   # Stats, Stale
├── content/
│   ├── content.go                 # Processor interface
│   ├── markdown.go                # MarkdownProcessor (code-block aware)
│   └── summarizer.go              # Summarizer interface + NoOpSummarizer
├── discovery/
│   ├── discovery.go               # Provider + ContentDiscoverer interfaces, Discoverer
│   ├── iface.go                   # ContentDiscoverer interface
│   ├── platform.go                # Doc platform detection (MkDocs, Docusaurus, Sphinx, GitBook)
│   ├── wellknown.go               # /llms.txt probing with JSON validation
│   ├── companion.go               # Companion file parsing
│   ├── context7.go                # Context7 API provider
│   └── sitemap.go                 # Sitemap-based discovery
├── mirror/
│   ├── mirror.go                  # Download, clean, convert, compare, store
│   ├── iface.go                   # Syncer interface
│   └── rewriter.go                # Link rewriting
├── store/
│   ├── store.go                   # Filesystem layout
│   ├── git.go                     # Git operations
│   ├── git_iface.go               # VersionStore interface
│   ├── index.go                   # SQLite FTS5 with path boosting, searchable summaries
│   ├── indexer.go                  # Indexer interface
│   └── categorize.go              # Categorizer interface + RuleCategorizer
├── fetcher/
│   ├── fetcher.go                 # HTTP client with rate limiting, ETags
│   ├── iface.go                   # HTTPFetcher interface
│   ├── convert.go                 # HTML→markdown with CSS cleaning + whitespace normalization
│   ├── cleaner.go                 # CSS selector-based content extraction
│   └── jsdetect.go                # JS/SPA shell detection
├── config/
│   └── config.go                  # YAML config loading
├── events/
│   ├── emitter.go                 # Event relay
│   └── iface.go                   # EventEmitter interface
├── internal/
│   ├── robots/                    # robots.txt checking (private)
│   └── lockfile/                  # Workspace concurrency lock (private)
├── cli/                           # Cobra CLI commands
├── mcp/
│   ├── server.go                  # MCP server, tracing, exported helpers
│   └── tools.go                   # 20 MCP tool handlers
├── skills/
│   ├── mcp.md                     # MCP tool usage guide for agents
│   └── cli.md                     # CLI usage guide
├── AGENT.md                       # Agent-facing documentation
├── DESIGN.md                      # This file
├── LICENSE                        # MIT
└── README.md                      # User-facing documentation
```

## Interfaces

Every Engine component is behind an interface. Defaults are constructed automatically; alternatives can be injected via `engine.WithXxx()` functional options.

| Interface | Package | Default | Swappable via |
|---|---|---|---|
| `HTTPFetcher` | `fetcher` | Rate-limited HTTP client | `WithFetcher()` |
| `Syncer` | `mirror` | Fetch → clean → convert → compare → store | `WithSyncer()` |
| `ContentDiscoverer` | `discovery` | Well-known + companions + sitemap + Context7 | `WithDiscovery()` |
| `Indexer` | `store` | SQLite FTS5 with path boosting | `WithIndexer()` |
| `VersionStore` | `store` | go-git | `WithGit()` |
| `EventEmitter` | `events` | HTTP event relay | `WithEvents()` |
| `Processor` | `content` | Markdown (code-block aware) | `WithProcessors()` |
| `Categorizer` | `store` | Rule-based (path + body heuristics) | `WithCategorizer()` |
| `Summarizer` | `content` | No-op (agent-submitted only) | — |
| `Provider` | `discovery` | SiteProvider, Context7Provider | `RegisterProvider()` |

### Library Usage

```go
import "github.com/dmoose/doctrove/engine"

eng, _ := engine.New(rootDir,
    engine.WithIndexer(customIndexer),
    engine.WithCategorizer(llmCategorizer),
    engine.WithFetcher(playwrightFetcher),
)
defer eng.Close()

// Use engine methods directly
result, _ := eng.Search(ctx, "authentication", "", "", "", "", 10, 0)
```

## Core Engine

```go
type Engine struct {
    Config      *config.Config
    Store       *store.Store
    Git         store.VersionStore
    Index       store.Indexer
    Discovery   discovery.ContentDiscoverer
    Mirror      mirror.Syncer
    Fetcher     fetcher.HTTPFetcher
    Events      events.EventEmitter
    Processors  []content.Processor
    Categorizer store.Categorizer
    RootDir     string
}
```

## Content Pipeline

The mirror/sync pipeline processes content through several stages:

```
HTTP Response
  → JS/SPA detection (skip shells that need browser rendering)
  → HTML cleaning (CSS selectors extract main content, strip nav/footer/sidebar)
  → HTML→Markdown conversion (whitespace normalization, quality check)
  → Link rewriting (absolute → relative)
  → Content comparison (Added/Updated/Unchanged classification)
  → Filesystem write + FTS index + git commit
```

## Content Discovery

1. **Well-known paths:** `/llms.txt`, `/llms-full.txt`, `/llms-ctx.txt`, `/llms-ctx-full.txt`, `/ai.txt`, `/.well-known/tdmrep.json`, `/.well-known/agent.json` (with JSON validation)
2. **Companion files:** Markdown links and bare URLs parsed from llms.txt
3. **Sitemap:** `sitemap.xml` paths containing `/llms/` or ending in `.md`/`.txt`
4. **Seed probing:** Common doc paths (`/docs`, `/getting-started`, `/introduction`, etc.) when no llms.txt found
5. **Platform detection:** Identifies MkDocs, Docusaurus, Sphinx, GitBook from HTML and returns optimized CSS selectors
6. **Context7 API:** Bare library names resolved to curated docs (optional)
7. **HTML conversion:** Sites serving HTML are cleaned and converted to markdown; JS-heavy SPA shells are skipped

## MCP Tools (20)

| Tool | Engine Method |
|---|---|
| `trove_discover` | `Discover()` |
| `trove_scan` | `Init()` + `SyncWithContentTypes()` |
| `trove_search` | `Search()` (with path boosting, path filter, total_count) |
| `trove_search_full` | `SearchFull()` |
| `trove_find` | `ListFiles()` filtered by path pattern |
| `trove_list` | `List()` |
| `trove_read` | `ReadSection()` |
| `trove_status` | `Status()` |
| `trove_diff` | `Diff()` (content-only, metadata filtered) |
| `trove_history` | `History()` |
| `trove_list_files` | `ListFiles()` (with category filter) |
| `trove_remove` | `Remove()` |
| `trove_catalog` | `Catalog()` (topics + category distribution) |
| `trove_stats` | `Stats()` |
| `trove_stale` | `Stale()` (threshold-based freshness check) |
| `trove_tag` | `Tag()` |
| `trove_refresh` | `Refresh()` (with warning visibility) |
| `trove_check` | `Check()` |
| `trove_outline` | `Outline()` |
| `trove_summarize` | `Summarize()` (re-indexes FTS for searchability) |

## Dependencies

- **cobra** — CLI framework
- **go-git/go-git** — git operations (no shell exec)
- **modernc.org/sqlite** — SQLite FTS5 (pure Go, CGo-free)
- **mark3labs/mcp-go** — MCP server (stdio transport)
- **gopkg.in/yaml.v3** — config
- **golang.org/x/time/rate** — rate limiting
- **JohannesKaufmann/html-to-markdown** — HTML conversion
- **PuerkitoBio/goquery** — HTML cleaning and platform detection
