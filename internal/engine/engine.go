package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dmoose/doctrove/internal/config"
	"github.com/dmoose/doctrove/internal/discovery"
	"github.com/dmoose/doctrove/internal/events"
	"github.com/dmoose/doctrove/internal/fetcher"
	"github.com/dmoose/doctrove/internal/lockfile"
	"github.com/dmoose/doctrove/internal/mirror"
	"github.com/dmoose/doctrove/internal/robots"
	"github.com/dmoose/doctrove/internal/store"
)

// Engine is the core orchestrator that ties all subsystems together.
type Engine struct {
	Config    *config.Config
	Store     *store.Store
	Git       *store.GitStore
	Index     store.Indexer
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

	em := events.New(cfg.Settings.EventsURL, "doctrove")

	disc := discovery.New(f, rc, cfg.Settings.MaxProbes)

	// Register additional providers based on config
	if cfg.Settings.Context7APIKey != "" {
		disc.RegisterProvider(discovery.NewContext7Provider(cfg.Settings.Context7APIKey))
	}

	return &Engine{
		Config:    cfg,
		Store:     s,
		Git:       gs,
		Index:     idx,
		Discovery: disc,
		Mirror:    mirror.New(f, s, idx),
		Fetcher:   f,
		Events:    em,
		RootDir:   rootDir,
	}, nil
}

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
	e.Events.EmitFull(events.Event{
		Channel: "sync",
		Action:  "init",
		Level:   "info",
		Data: map[string]any{
			"domain":      info.Domain,
			"files_found": info.FileCount,
			"provider":    e.providerFor(rawURL),
		},
	})
	return info, nil
}

// Sync downloads/updates content for a site.
func (e *Engine) Sync(ctx context.Context, domain string) (*SyncResult, error) {
	start := time.Now()
	lock, err := lockfile.Acquire(e.RootDir)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

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
	syncLevel := "info"
	if len(mr.Errors) > 0 {
		syncLevel = "warn"
	}
	e.Events.EmitFull(events.Event{
		Channel:    "sync",
		Action:     "sync",
		Level:      syncLevel,
		DurationMS: time.Since(start).Milliseconds(),
		Data: map[string]any{
			"domain":    domain,
			"added":     len(mr.Added),
			"unchanged": len(mr.Unchanged),
			"skipped":   len(mr.Skipped),
			"errors":    len(mr.Errors),
			"provider":  e.providerFor(siteCfg.URL),
		},
	})
	return sr, nil
}

// parseContentTypes splits a comma-separated content types string into a
// set for filtering. Returns nil if empty (meaning "allow all").
func parseContentTypes(contentTypes string) map[string]bool {
	if contentTypes == "" {
		return nil
	}
	allowed := make(map[string]bool)
	for _, ct := range strings.Split(contentTypes, ",") {
		ct = strings.TrimSpace(ct)
		if ct != "" {
			allowed[ct] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

// SyncWithContentTypes syncs only files matching the given content types
// (comma-separated, e.g. "llms-txt,llms-full-txt"). This lets agents skip
// companion pages when they only need the index files. The filter is persisted
// in the site config so that Refresh honours it.
func (e *Engine) SyncWithContentTypes(ctx context.Context, domain, contentTypes string) (*SyncResult, error) {
	allowed := parseContentTypes(contentTypes)
	if allowed == nil {
		return e.Sync(ctx, domain)
	}

	start := time.Now()
	lock, err := lockfile.Acquire(e.RootDir)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked — run init first", domain)
	}

	// Persist content_types so Refresh honours the same filter
	siteCfg.ContentTypes = contentTypes

	result, err := e.Discovery.Discover(ctx, siteCfg.URL)
	if err != nil {
		return nil, fmt.Errorf("discovering content for %s: %w", domain, err)
	}

	// Filter discovered files to only allowed content types
	var filtered []discovery.DiscoveredFile
	for _, f := range result.Files {
		if allowed[string(f.ContentType)] {
			filtered = append(filtered, f)
		}
	}
	result.Files = filtered

	// Build include/exclude filter from site config
	filter := mirror.BuildFilter(siteCfg.Include, siteCfg.Exclude)

	mr, err := e.Mirror.Sync(ctx, result, filter)
	if err != nil {
		return nil, fmt.Errorf("syncing %s: %w", domain, err)
	}

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

	commitMsg := fmt.Sprintf("sync %s: %d files (filtered)", domain, len(mr.Added))
	committed, err := e.Git.Commit(commitMsg)
	if err != nil {
		return nil, fmt.Errorf("committing changes for %s: %w", domain, err)
	}

	sr := &SyncResult{
		SyncResult: *mr,
		SyncTime:   now,
		Committed:  committed,
	}
	syncLevel := "info"
	if len(mr.Errors) > 0 {
		syncLevel = "warn"
	}
	e.Events.EmitFull(events.Event{
		Channel:    "sync",
		Action:     "sync",
		Level:      syncLevel,
		DurationMS: time.Since(start).Milliseconds(),
		Data: map[string]any{
			"domain":        domain,
			"added":         len(mr.Added),
			"unchanged":     len(mr.Unchanged),
			"skipped":       len(mr.Skipped),
			"errors":        len(mr.Errors),
			"content_types": contentTypes,
			"provider":      e.providerFor(siteCfg.URL),
		},
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
	result, err := e.Discovery.Discover(ctx, rawURL)
	if err == nil && result != nil {
		e.Events.EmitFull(events.Event{
			Channel: "sync",
			Action:  "discover",
			Level:   "info",
			Data: map[string]any{
				"domain":      result.Domain,
				"files_found": len(result.Files),
				"provider":    e.providerFor(rawURL),
			},
		})
	}
	return result, err
}

// providerFor returns the name of the provider that would handle this input.
func (e *Engine) providerFor(input string) string {
	for _, p := range e.Discovery.Providers() {
		if p.CanHandle(input) {
			return p.Name()
		}
	}
	return "none"
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

// humanAge is defined in stats.go

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
func (e *Engine) Search(ctx context.Context, query string, site, contentType, category string, limit, offset int) (*SearchResult, error) {
	hits, err := e.Index.Search(query, store.SearchOpts{
		Site:        site,
		ContentType: contentType,
		Category:    category,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}

	result := &SearchResult{Hits: hits}
	if len(hits) == 0 {
		result.Suggestion = "No local results. Use trove_discover to check if a URL has LLM content, or trove_scan to add and sync a site."
	}
	return result, nil
}

// SearchFull searches and returns the full content of the top hit.
func (e *Engine) SearchFull(ctx context.Context, query string, site, contentType, category string) (*SearchFullResult, error) {
	sr, err := e.Search(ctx, query, site, contentType, category, 1, 0)
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
	result.Category = hit.Category
	result.Content = string(body)
	return result, nil
}

// SearchFullResult contains the full content of the best search match.
type SearchFullResult struct {
	Domain      string `json:"domain,omitempty"`
	Path        string `json:"path,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Category    string `json:"category,omitempty"`
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
		ct := store.ClassifyPath(urlPath)
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

	e.Events.EmitFull(events.Event{
		Channel: "sync",
		Action:  "remove",
		Level:   "info",
		Data:    map[string]any{"domain": domain, "keep_files": keepFiles},
	})
	return nil
}

// Tag overrides the category for a specific file (agent feedback).
func (e *Engine) Tag(ctx context.Context, domain, path, category string) error {
	return e.Index.SetCategory(domain, path, category)
}

// Refresh re-syncs a tracked site. If content_types was set during the
// original scan, Refresh honours that filter so it doesn't pull content
// the agent intentionally excluded. Uses cached ETags for conditional
// fetching.
func (e *Engine) Refresh(ctx context.Context, domain string) (*SyncResult, error) {
	siteCfg, ok := e.Config.Sites[domain]
	if !ok {
		return nil, fmt.Errorf("site %q not tracked", domain)
	}
	if siteCfg.ContentTypes != "" {
		return e.SyncWithContentTypes(ctx, domain, siteCfg.ContentTypes)
	}
	return e.Sync(ctx, domain)
}

// OutlineEntry represents a heading in a document's structure.
type OutlineEntry struct {
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	Line    int    `json:"line"`
	Size    int    `json:"size_chars"` // characters in this section (until next heading of same/higher level)
}

// OutlineResult is the table of contents for a file.
type OutlineResult struct {
	Domain    string         `json:"domain"`
	Path      string         `json:"path"`
	TotalSize int            `json:"total_size"`
	Summary   string         `json:"summary,omitempty"`
	Sections  []OutlineEntry `json:"sections"`
}

// Outline parses a mirrored file and returns its heading structure.
func (e *Engine) Outline(ctx context.Context, domain, path string) (*OutlineResult, error) {
	body, err := e.Store.ReadContent(domain, path)
	if err != nil {
		return nil, fmt.Errorf("reading %s%s: %w", domain, path, err)
	}

	content := string(body)
	lines := strings.Split(content, "\n")

	var sections []OutlineEntry
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for _, c := range trimmed {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		if level > 0 && level <= 6 {
			heading := strings.TrimSpace(trimmed[level:])
			if heading != "" {
				sections = append(sections, OutlineEntry{
					Heading: heading,
					Level:   level,
					Line:    i + 1, // 1-based
				})
			}
		}
	}

	// Compute section sizes: chars from this heading to the next heading of same/higher level
	for i := range sections {
		startLine := sections[i].Line - 1 // back to 0-based
		var endLine int
		if i+1 < len(sections) {
			endLine = sections[i+1].Line - 1
		} else {
			endLine = len(lines)
		}
		size := 0
		for l := startLine; l < endLine && l < len(lines); l++ {
			size += len(lines[l]) + 1
		}
		sections[i].Size = size
	}

	summary, _, _ := e.Index.GetSummary(domain, path)

	return &OutlineResult{
		Domain:    domain,
		Path:      path,
		TotalSize: len(content),
		Summary:   summary,
		Sections:  sections,
	}, nil
}

// ReadSection reads a specific section of a file by heading match, or a line range.
// If section is non-empty, returns content from that heading to the next heading of same/higher level.
// If section is empty and maxLines > 0, returns the first maxLines lines.
func (e *Engine) ReadSection(ctx context.Context, domain, path, section string, maxLines int) (string, error) {
	body, err := e.Store.ReadContent(domain, path)
	if err != nil {
		return "", fmt.Errorf("reading %s%s: %w", domain, path, err)
	}

	content := string(body)
	if section == "" && maxLines <= 0 {
		return content, nil
	}

	lines := strings.Split(content, "\n")

	if section != "" {
		sectionLower := strings.ToLower(section)
		startIdx := -1
		startLevel := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "#") {
				continue
			}
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			heading := strings.TrimSpace(trimmed[level:])
			if startIdx == -1 {
				if strings.Contains(strings.ToLower(heading), sectionLower) {
					startIdx = i
					startLevel = level
				}
			} else {
				// Found the start — look for end (same or higher level heading)
				if level <= startLevel {
					result := strings.Join(lines[startIdx:i], "\n")
					return result, nil
				}
			}
		}
		if startIdx >= 0 {
			return strings.Join(lines[startIdx:], "\n"), nil
		}
		return "", fmt.Errorf("section %q not found in %s%s", section, domain, path)
	}

	// Line-limited read
	if maxLines > len(lines) {
		maxLines = len(lines)
	}
	return strings.Join(lines[:maxLines], "\n"), nil
}

// Summarize stores an agent-submitted summary for a file.
func (e *Engine) Summarize(ctx context.Context, domain, path, summary string) error {
	return e.Index.SetSummary(domain, path, summary)
}

// Close releases resources held by the engine.
func (e *Engine) Close() error {
	e.Events.Flush()
	return e.Index.Close()
}
