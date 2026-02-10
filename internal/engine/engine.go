package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dmoose/llmshadow/internal/config"
	"github.com/dmoose/llmshadow/internal/discovery"
	"github.com/dmoose/llmshadow/internal/events"
	"github.com/dmoose/llmshadow/internal/fetcher"
	"github.com/dmoose/llmshadow/internal/mirror"
	"github.com/dmoose/llmshadow/internal/robots"
	"github.com/dmoose/llmshadow/internal/store"
)

// Engine is the core orchestrator that ties all subsystems together.
type Engine struct {
	Config    *config.Config
	Store     *store.Store
	Git       *store.GitStore
	Index     *store.Index
	Discovery *discovery.Discoverer
	Mirror    *mirror.Mirror
	Fetcher   *fetcher.Fetcher
	Events    *events.Emitter
	RootDir   string
}

// Options configures engine behavior.
type Options struct {
	RespectRobots bool
}

// New creates an Engine rooted at the given directory.
func New(rootDir string, opts ...Options) (*Engine, error) {
	cfg, err := config.Load(rootDir)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	f := fetcher.New(fetcher.Options{
		UserAgent:    cfg.Settings.UserAgent,
		RatePerHost:  cfg.Settings.RateLimit,
		BurstPerHost: cfg.Settings.RateBurst,
		Timeout:      cfg.Settings.TimeoutDuration(),
	})
	s := store.New(rootDir)

	gs, err := store.InitGit(rootDir)
	if err != nil {
		return nil, fmt.Errorf("initializing git: %w", err)
	}

	idx, err := store.OpenIndex(rootDir)
	if err != nil {
		return nil, fmt.Errorf("opening search index: %w", err)
	}

	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	var rc *robots.Checker
	if o.RespectRobots {
		rc = robots.New(f)
	}

	em := events.New(cfg.Settings.EventsURL, "llmshadow")

	return &Engine{
		Config:    cfg,
		Store:     s,
		Git:       gs,
		Index:     idx,
		Discovery: discovery.New(f, rc, cfg.Settings.MaxProbes),
		Mirror:    mirror.New(f, s),
		Fetcher:   f,
		Events:    em,
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
	Available []string `json:"available"`
}

// ChangeEntry is a git log entry.
type ChangeEntry = store.LogEntry

// FileEntry describes a file in a site's mirror.
type FileEntry struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Init adds a new site to track. It probes for content but does not download yet.
func (e *Engine) Init(ctx context.Context, rawURL string) (*SiteInfo, error) {
	if rawURL[len(rawURL)-1] == '/' {
		rawURL = rawURL[:len(rawURL)-1]
	}

	result, err := e.Discovery.Discover(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("discovering content: %w", err)
	}

	if err := e.Config.AddSite(result.Domain, rawURL); err != nil {
		return nil, err
	}

	if err := e.Store.EnsureSiteDir(result.Domain); err != nil {
		return nil, fmt.Errorf("creating site directory: %w", err)
	}

	if err := e.Config.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	info := &SiteInfo{
		Domain:    result.Domain,
		URL:       rawURL,
		FileCount: len(result.Files),
	}
	e.Events.Emit("init", "cli", map[string]any{"domain": info.Domain, "files_found": info.FileCount})
	return info, nil
}

// Sync downloads/updates content for a site.
func (e *Engine) Sync(ctx context.Context, domain string) (*SyncResult, error) {
	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked — run init first", domain)
	}

	result, err := e.Discovery.Discover(ctx, siteCfg.URL)
	if err != nil {
		return nil, fmt.Errorf("discovering content for %s: %w", domain, err)
	}

	// Build include/exclude filter from site config
	filter := mirror.BuildFilter(siteCfg.Include, siteCfg.Exclude)

	mr, err := e.Mirror.Sync(ctx, result, filter)
	if err != nil {
		return nil, fmt.Errorf("syncing %s: %w", domain, err)
	}

	// Update search index for synced files
	for _, file := range result.Files {
		body, readErr := e.Store.ReadContent(domain, file.Path)
		if readErr != nil {
			continue
		}
		_ = e.Index.IndexFile(domain, file.Path, string(file.ContentType), string(body))
	}

	now := time.Now()
	e.Config.UpdateLastSync(domain, now)
	if err := e.Config.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	commitMsg := fmt.Sprintf("sync %s: %d files", domain, len(mr.Added))
	committed, err := e.Git.Commit(commitMsg)
	if err != nil {
		return nil, fmt.Errorf("committing changes for %s: %w", domain, err)
	}

	sr := &SyncResult{
		SyncResult: *mr,
		SyncTime:   now,
		Committed:  committed,
	}
	e.Events.Emit("sync", "cli", map[string]any{
		"domain":    domain,
		"files":     len(mr.Added),
		"errors":    len(mr.Errors),
	})
	return sr, nil
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

// SearchHit re-exports store.SearchHit.
type SearchHit = store.SearchHit

// SearchResult wraps search hits with metadata.
type SearchResult struct {
	Hits       []SearchHit `json:"results"`
	Suggestion string      `json:"suggestion,omitempty"`
}

// Search performs a full-text search across indexed content.
func (e *Engine) Search(ctx context.Context, query string, site, contentType string, limit int) (*SearchResult, error) {
	hits, err := e.Index.Search(query, store.SearchOpts{
		Site:        site,
		ContentType: contentType,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}

	result := &SearchResult{Hits: hits}
	if len(hits) == 0 {
		result.Suggestion = "No local results. Use shadow_discover to check if a URL has LLM content, or shadow_scan to add and sync a site."
	}
	return result, nil
}

// SearchFull searches and returns the full content of the top hit.
func (e *Engine) SearchFull(ctx context.Context, query string, site, contentType string) (*SearchFullResult, error) {
	sr, err := e.Search(ctx, query, site, contentType, 1)
	if err != nil {
		return nil, err
	}

	result := &SearchFullResult{Suggestion: sr.Suggestion}
	if len(sr.Hits) == 0 {
		return result, nil
	}

	hit := sr.Hits[0]
	body, err := e.Store.ReadContent(hit.Domain, hit.Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s%s: %w", hit.Domain, hit.Path, err)
	}

	result.Domain = hit.Domain
	result.Path = hit.Path
	result.ContentType = hit.ContentType
	result.Content = string(body)
	return result, nil
}

// SearchFullResult contains the full content of the best search match.
type SearchFullResult struct {
	Domain      string `json:"domain,omitempty"`
	Path        string `json:"path,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Content     string `json:"content,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// RebuildIndex rebuilds the search index from files on disk.
func (e *Engine) RebuildIndex(ctx context.Context) error {
	return e.Index.Rebuild(e.Store)
}

// ListFiles returns all content files for a tracked site.
func (e *Engine) ListFiles(ctx context.Context, domain string) ([]FileEntry, error) {
	if _, ok := e.Config.Sites[domain]; !ok {
		return nil, fmt.Errorf("site %q not tracked", domain)
	}

	siteDir := e.Store.SiteDir(domain)
	var files []FileEntry
	err := filepath.Walk(siteDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "_meta" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(siteDir, path)
		urlPath := "/" + rel
		files = append(files, FileEntry{
			Path:        urlPath,
			Size:        info.Size(),
			ContentType: store.ClassifyPath(urlPath),
		})
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing files for %s: %w", domain, err)
	}
	return files, nil
}

// Remove untracks a site, optionally deleting its files.
func (e *Engine) Remove(ctx context.Context, domain string, keepFiles bool) error {
	if err := e.Config.RemoveSite(domain); err != nil {
		return err
	}

	if !keepFiles {
		siteDir := e.Store.SiteDir(domain)
		if err := os.RemoveAll(siteDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing files for %s: %w", domain, err)
		}
	}

	// Clean search index
	_ = e.Index.DeleteSite(domain)

	if err := e.Config.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	commitMsg := fmt.Sprintf("remove %s", domain)
	_, _ = e.Git.Commit(commitMsg)

	return nil
}

// Close releases resources held by the engine.
func (e *Engine) Close() error {
	return e.Index.Close()
}
