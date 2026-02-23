# doctrove Design Document

A Go tool that discovers, downloads, and maintains local mirrors of websites' LLM-targeted content (llms.txt, companion .html.md files, etc.) with git-based change tracking, full-text search, and an MCP interface for agent access.

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
│  │  │   Search   │ │   Git    │ │  Config   │ │ │
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
├── cmd/
│   └── doctrove/
│       └── main.go              # Entry point
├── internal/
│   ├── engine/
│   │   └── engine.go            # Core orchestrator — ties subsystems together
│   ├── discovery/
│   │   ├── discovery.go         # Discovery strategy interface + orchestrator
│   │   ├── wellknown.go         # /llms.txt, /llms-full.txt, /ai.txt probing
│   │   ├── companion.go         # .html.md companion file parsing
│   │   └── sitemap.go           # Sitemap-based discovery
│   ├── mirror/
│   │   ├── mirror.go            # Download, store, link-rewrite
│   │   ├── rewriter.go          # URL → local path rewriting
│   │   └── diff.go              # Content diffing before commit
│   ├── store/
│   │   ├── store.go             # Filesystem layout abstraction
│   │   ├── git.go               # Git commit/log/diff operations
│   │   └── index.go             # SQLite FTS5 search index
│   ├── config/
│   │   └── config.go            # YAML config loading, defaults, validation
│   ├── search/
│   │   └── search.go            # Full-text + metadata search over mirrored content
│   └── fetcher/
│       └── fetcher.go           # HTTP client: rate limiting, ETags, retries
├── cli/
│   ├── root.go                  # cobra root command
│   ├── init.go                  # doctrove init <url>
│   ├── sync.go                  # doctrove sync [site|--all]
│   ├── status.go                # doctrove status [site]
│   ├── search.go                # doctrove search <query> [--site]
│   ├── list.go                  # doctrove list [--format json|table]
│   ├── serve.go                 # doctrove serve (local HTTP)
│   ├── check.go                 # doctrove check [site] (dry-run sync)
│   ├── discover.go              # doctrove discover <url>
│   ├── history.go               # doctrove history [site]
│   ├── diff.go                  # doctrove diff <site> [ref..ref]
│   └── mcp.go                   # doctrove mcp (start MCP server)
├── mcp/
│   ├── server.go                # MCP server setup, tool registration
│   └── tools.go                 # MCP tool handlers → engine calls
├── go.mod
├── go.sum
└── doctrove.yaml                # Example config
```

## Core Engine Interface

The single source of truth that both CLI and MCP call into. Every method returns structured data — CLI formats for humans, MCP returns JSON.

```go
package engine

type Engine struct {
    Config    *config.Config
    Store     *store.Store
    Discovery *discovery.Discoverer
    Mirror    *mirror.Mirror
    Search    *search.Searcher
    Fetcher   *fetcher.Fetcher
}

// Site lifecycle
func (e *Engine) Init(ctx context.Context, url string, opts InitOpts) (*SiteInfo, error)
func (e *Engine) Remove(ctx context.Context, site string) error

// Content operations
func (e *Engine) Sync(ctx context.Context, site string, opts SyncOpts) (*SyncResult, error)
func (e *Engine) SyncAll(ctx context.Context, opts SyncOpts) ([]SyncResult, error)
func (e *Engine) Check(ctx context.Context, site string) (*CheckResult, error)

// Discovery (standalone — probe without committing to tracking)
func (e *Engine) Discover(ctx context.Context, url string) (*DiscoveryResult, error)

// Query
func (e *Engine) Status(ctx context.Context, site string) (*SiteStatus, error)
func (e *Engine) List(ctx context.Context) ([]SiteInfo, error)
func (e *Engine) Search(ctx context.Context, query string, opts SearchOpts) ([]SearchHit, error)

// History
func (e *Engine) History(ctx context.Context, site string, opts HistoryOpts) ([]ChangeEntry, error)
func (e *Engine) Diff(ctx context.Context, site string, from, to string) (string, error)
```

## Discovery Strategy

Following the llms.txt ecosystem patterns:

1. **Well-known locations**: `/llms.txt`, `/llms-full.txt`, `/ai.txt`
2. **Companion file discovery**: Parse llms.txt for `.html.md` file references
3. **Link following**: Parse llms.txt sections to find all referenced content
4. **Sitemap integration**: Check for structured LLM content organization

Discovery is a standalone subsystem. An agent can call `Discover()` to probe a URL without tracking it. `Init()` + `Sync()` persists results.

## Storage

### Filesystem Layout

```
<root>/
├── .git/                        # Git repository for change tracking
├── doctrove.yaml                # Configuration
├── doctrove.db                  # SQLite FTS5 search index
├── sites/
│   └── <domain>/
│       ├── llms.txt             # Main index file
│       ├── llms-full.txt        # Full content export
│       ├── docs/
│       │   ├── api.html.md      # Companion files
│       │   └── guide.html.md
│       └── _meta/
│           ├── discovered.json  # Discovery metadata
│           └── links.json       # Link mapping
└── index.md                     # Global index of all sites
```

### Store Abstraction

The `store` package owns the directory convention. Nothing else touches the filesystem directly, so the layout can evolve without touching engine logic.

### Git Integration

Each sync auto-commits changes. Git is the source of truth; the search index is derived and can be rebuilt from the filesystem at any time.

### Search Index

SQLite FTS5 — chosen over bleve because it handles concurrent access from CLI and MCP processes cleanly. The index rebuilds from the filesystem on demand.

## Fetcher

All HTTP goes through `fetcher.Fetcher`:

- Per-domain rate limiting (x/time/rate)
- robots.txt respect (configurable)
- ETag / Last-Modified conditional requests for efficient syncs
- Retries with exponential backoff
- User-Agent identification (`doctrove/<version>`)

## MCP Interface

MCP server runs via `doctrove mcp` using stdio transport. Agents add it to their MCP config directly.

### Tools Exposed

| Tool | Description | Engine Method |
|---|---|---|
| `trove_discover` | Probe a URL for LLM content without saving | `Engine.Discover()` |
| `trove_scan` | Init + sync a new site | `Engine.Init()` + `Engine.Sync()` |
| `trove_search` | Full-text search across mirrored content | `Engine.Search()` |
| `trove_list` | List all tracked sites with status | `Engine.List()` |
| `trove_read` | Read a specific mirrored file | direct file read via `Store` |
| `trove_status` | Get sync status and stats for a site | `Engine.Status()` |
| `trove_diff` | Show what changed between syncs | `Engine.Diff()` |

## CLI Commands

```
doctrove init <url>              # Add a site to track
doctrove sync [site|--all]       # Download/update content
doctrove check [site|--all]      # Dry-run: report what would change
doctrove status [site]           # Show tracked sites, last sync, file counts
doctrove list [--format json]    # List all tracked sites
doctrove search <query>          # Full-text search across all content
  --site <domain>                #   scope to one site
  --type llms-txt|companion|full #   filter by content type
doctrove discover <url>          # Probe a URL without tracking
doctrove history [site]          # Show git-based change history
doctrove diff <site> [ref..ref]  # Show content changes between syncs
doctrove serve [--port 8080]     # Serve mirrored content over HTTP
doctrove mcp                     # Start MCP server (stdio)
doctrove config                  # Show/edit config
```

## Configuration

```yaml
sites:
  example.com:
    url: "https://example.com"
    include:
      - "/llms*.txt"
      - "/**/*.html.md"
      - "/docs/**"
    exclude:
      - "/internal/**"
      - "/admin/**"
    update_freq: "daily"
    last_sync: "2026-03-16T10:30:00Z"
```

## Dependencies

- **cobra** — CLI framework
- **go-git** — git operations without shelling out
- **modernc.org/sqlite** or **mattn/go-sqlite3** — SQLite FTS5 (pure Go preferred)
- **mcp-go** (or raw JSON-RPC over stdio) — MCP server
- **gopkg.in/yaml.v3** — config
- **golang.org/x/time/rate** — rate limiting

## Build Order

Each phase delivers a usable tool:

### Phase 1 — Core loop
`config`, `fetcher`, `discovery/wellknown`, `store` (filesystem only), `mirror`, `engine`.
CLI: `init`, `sync`, `status`, `list`.
No git, no search — just download files to disk.

### Phase 2 — Git + history
`store/git`, `engine.History`, `engine.Diff`.
CLI: `history`, `diff`, `check`.
Each sync auto-commits.

### Phase 3 — Search
`store/index`, `search`.
CLI: `search`.
Index builds on sync.

### Phase 4 — MCP
`mcp/` package. Wraps engine methods as MCP tools.
CLI: `mcp`.

### Phase 5 — Polish
`serve`, companion file discovery, sitemap integration, conditional fetching (ETags), `--format` flags, `discover` command.
