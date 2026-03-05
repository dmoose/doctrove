package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dmoose/doctrove/discovery"
	"github.com/dmoose/doctrove/internal/lockfile"
	"github.com/dmoose/doctrove/mirror"
	"github.com/dmoose/doctrove/store"
)

// SiteInfo is returned when listing or describing a site.
type SiteInfo struct {
	Domain         string         `json:"domain"`
	URL            string         `json:"url"`
	LastSync       time.Time      `json:"last_sync"`
	FileCount      int            `json:"file_count"`
	ContentTypes   string         `json:"content_types,omitempty"`
	Age            string         `json:"age,omitempty"`
	CategoryCounts map[string]int `json:"categories,omitempty"`
}

// SyncResult wraps the mirror result with config updates.
type SyncResult struct {
	mirror.SyncResult
	SyncTime  time.Time `json:"sync_time"`
	Committed bool      `json:"committed"`
}

// CheckFile describes a file available for syncing.
type CheckFile struct {
	Path        string `json:"path"`
	Size        int    `json:"size"`
	ContentType string `json:"content_type"`
}

// CheckResult reports what would change without downloading.
type CheckResult struct {
	Domain    string      `json:"domain"`
	Available []CheckFile `json:"available"`
}

// ChangeEntry is a git log entry.
type ChangeEntry = store.LogEntry

// FileEntry describes a file in a site's mirror.
type FileEntry struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Category    string `json:"category"`
}

// Init adds a new site to track. It probes for content but does not download yet.
func (e *Engine) Init(ctx context.Context, rawURL string) (*SiteInfo, error) {
	lock, err := lockfile.Acquire(e.RootDir)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	if len(rawURL) > 0 && rawURL[len(rawURL)-1] == '/' {
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
	return info, nil
}

// Discover probes a URL for LLM content without tracking it.
func (e *Engine) Discover(ctx context.Context, rawURL string) (*discovery.Result, error) {
	return e.Discovery.Discover(ctx, rawURL)
}

// Status returns info about a tracked site including category breakdown and age.
func (e *Engine) Status(ctx context.Context, domain string) (*SiteInfo, error) {
	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked", domain)
	}

	count, err := e.Store.SiteFileCount(domain)
	if err != nil {
		return nil, fmt.Errorf("counting files: %w", err)
	}

	cats, _ := e.Index.CategoryCounts(domain)

	var age string
	if !siteCfg.LastSync.IsZero() {
		age = humanAge(time.Since(siteCfg.LastSync))
	}

	return &SiteInfo{
		Domain:         domain,
		URL:            siteCfg.URL,
		LastSync:       siteCfg.LastSync,
		FileCount:      count,
		ContentTypes:   siteCfg.ContentTypes,
		Age:            age,
		CategoryCounts: cats,
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

	var files []CheckFile
	for _, f := range result.Files {
		files = append(files, CheckFile{
			Path:        f.Path,
			Size:        f.Size,
			ContentType: string(f.ContentType),
		})
	}

	return &CheckResult{
		Domain:    domain,
		Available: files,
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
		ct, _ := e.Index.GetContentType(domain, urlPath)
		if ct == "" {
			ct = store.ClassifyPath(urlPath)
		}
		cat, _ := e.Index.GetCategory(domain, urlPath)
		files = append(files, FileEntry{
			Path:        urlPath,
			Size:        info.Size(),
			ContentType: ct,
			Category:    cat,
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
	lock, err := lockfile.Acquire(e.RootDir)
	if err != nil {
		return err
	}
	defer lock.Release()

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

// Tag overrides the category for a specific file (agent feedback).
func (e *Engine) Tag(ctx context.Context, domain, path, category string) error {
	return e.Index.SetCategory(domain, path, category)
}

// Summarize stores an agent-submitted summary for a file.
func (e *Engine) Summarize(ctx context.Context, domain, path, summary string) error {
	return e.Index.SetSummary(domain, path, summary)
}
