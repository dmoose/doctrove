package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/dmoose/llmshadow/internal/config"
	"github.com/dmoose/llmshadow/internal/discovery"
	"github.com/dmoose/llmshadow/internal/fetcher"
	"github.com/dmoose/llmshadow/internal/mirror"
	"github.com/dmoose/llmshadow/internal/store"
)

// Engine is the core orchestrator that ties all subsystems together.
type Engine struct {
	Config    *config.Config
	Store     *store.Store
	Git       *store.GitStore
	Discovery *discovery.Discoverer
	Mirror    *mirror.Mirror
	Fetcher   *fetcher.Fetcher
	RootDir   string
}

// New creates an Engine rooted at the given directory.
func New(rootDir string) (*Engine, error) {
	cfg, err := config.Load(rootDir)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	f := fetcher.New()
	s := store.New(rootDir)

	gs, err := store.InitGit(rootDir)
	if err != nil {
		return nil, fmt.Errorf("initializing git: %w", err)
	}

	return &Engine{
		Config:    cfg,
		Store:     s,
		Git:       gs,
		Discovery: discovery.New(f),
		Mirror:    mirror.New(f, s),
		Fetcher:   f,
		RootDir:   rootDir,
	}, nil
}

// SiteInfo is returned when listing or describing a site.
type SiteInfo struct {
	Domain    string    `json:"domain"`
	URL       string    `json:"url"`
	LastSync  time.Time `json:"last_sync,omitempty"`
	FileCount int       `json:"file_count"`
}

// SyncResult wraps the mirror result with config updates.
type SyncResult struct {
	mirror.SyncResult
	SyncTime  time.Time `json:"sync_time"`
	Committed bool      `json:"committed"`
}

// CheckResult reports what would change without downloading.
type CheckResult struct {
	Domain    string   `json:"domain"`
	Available []string `json:"available"` // Files that exist remotely
}

// ChangeEntry is a git log entry.
type ChangeEntry = store.LogEntry

// Init adds a new site to track. It probes for content but does not download yet.
func (e *Engine) Init(ctx context.Context, rawURL string) (*SiteInfo, error) {
	// Normalize URL
	if rawURL[len(rawURL)-1] == '/' {
		rawURL = rawURL[:len(rawURL)-1]
	}

	// Discover to validate the site has LLM content
	result, err := e.Discovery.Discover(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("discovering content: %w", err)
	}

	// Add to config
	if err := e.Config.AddSite(result.Domain, rawURL); err != nil {
		return nil, err
	}

	// Create directory structure
	if err := e.Store.EnsureSiteDir(result.Domain); err != nil {
		return nil, fmt.Errorf("creating site directory: %w", err)
	}

	// Save config
	if err := e.Config.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	return &SiteInfo{
		Domain:    result.Domain,
		URL:       rawURL,
		FileCount: len(result.Files),
	}, nil
}

// Sync downloads/updates content for a site.
func (e *Engine) Sync(ctx context.Context, domain string) (*SyncResult, error) {
	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked — run init first", domain)
	}

	// Discover current content
	result, err := e.Discovery.Discover(ctx, siteCfg.URL)
	if err != nil {
		return nil, fmt.Errorf("discovering content for %s: %w", domain, err)
	}

	// Mirror the content
	mr, err := e.Mirror.Sync(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("syncing %s: %w", domain, err)
	}

	// Update config with sync time
	now := time.Now()
	e.Config.UpdateLastSync(domain, now)
	if err := e.Config.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	// Auto-commit changes
	commitMsg := fmt.Sprintf("sync %s: %d files", domain, len(mr.Added))
	committed, err := e.Git.Commit(commitMsg)
	if err != nil {
		return nil, fmt.Errorf("committing changes for %s: %w", domain, err)
	}

	return &SyncResult{
		SyncResult: *mr,
		SyncTime:   now,
		Committed:  committed,
	}, nil
}

// SyncAll syncs all tracked sites.
func (e *Engine) SyncAll(ctx context.Context) ([]SyncResult, error) {
	var results []SyncResult
	for domain := range e.Config.Sites {
		r, err := e.Sync(ctx, domain)
		if err != nil {
			results = append(results, SyncResult{
				SyncResult: mirror.SyncResult{
					Domain: domain,
					Errors: []string{err.Error()},
				},
			})
			continue
		}
		results = append(results, *r)
	}
	return results, nil
}

// Discover probes a URL for LLM content without tracking it.
func (e *Engine) Discover(ctx context.Context, rawURL string) (*discovery.Result, error) {
	return e.Discovery.Discover(ctx, rawURL)
}

// Status returns info about a tracked site.
func (e *Engine) Status(ctx context.Context, domain string) (*SiteInfo, error) {
	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked", domain)
	}

	count, err := e.Store.SiteFileCount(domain)
	if err != nil {
		return nil, fmt.Errorf("counting files: %w", err)
	}

	return &SiteInfo{
		Domain:    domain,
		URL:       siteCfg.URL,
		LastSync:  siteCfg.LastSync,
		FileCount: count,
	}, nil
}

// List returns info about all tracked sites.
func (e *Engine) List(ctx context.Context) ([]SiteInfo, error) {
	var sites []SiteInfo
	for domain, siteCfg := range e.Config.Sites {
		count, _ := e.Store.SiteFileCount(domain)
		sites = append(sites, SiteInfo{
			Domain:    domain,
			URL:       siteCfg.URL,
			LastSync:  siteCfg.LastSync,
			FileCount: count,
		})
	}
	return sites, nil
}

// Check probes a site for available content without downloading (dry-run).
func (e *Engine) Check(ctx context.Context, domain string) (*CheckResult, error) {
	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked", domain)
	}

	result, err := e.Discovery.Discover(ctx, siteCfg.URL)
	if err != nil {
		return nil, fmt.Errorf("discovering content for %s: %w", domain, err)
	}

	var paths []string
	for _, f := range result.Files {
		paths = append(paths, f.Path)
	}

	return &CheckResult{
		Domain:    domain,
		Available: paths,
	}, nil
}

// History returns recent change entries from git, optionally filtered to a site.
func (e *Engine) History(ctx context.Context, site string, limit int) ([]ChangeEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	return e.Git.Log(site, limit)
}

// Diff returns the diff between two git refs.
func (e *Engine) Diff(ctx context.Context, from, to string) (string, error) {
	if to == "" {
		to = "HEAD"
	}
	return e.Git.Diff(from, to)
}
