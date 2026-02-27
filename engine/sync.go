package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dmoose/doctrove/discovery"
	"github.com/dmoose/doctrove/internal/lockfile"
	"github.com/dmoose/doctrove/mirror"
)

// Sync downloads/updates content for a site.
func (e *Engine) Sync(ctx context.Context, domain string) (*SyncResult, error) {
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
		cat := e.Categorizer.Categorize(domain, file.Path, string(file.ContentType), string(body))
		_ = e.Index.IndexFile(domain, file.Path, string(file.ContentType), string(body), cat)
	}

	now := time.Now()
	e.Config.UpdateLastSync(domain, now)
	if err := e.Config.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	commitMsg := fmt.Sprintf("sync %s: %d files", domain, len(mr.Added))
	committed, err := e.Git.Commit(commitMsg)
	if err != nil {
		mr.Errors = append(mr.Errors, fmt.Sprintf("git commit: %v", err))
	}

	sr := &SyncResult{
		SyncResult: *mr,
		SyncTime:   now,
		Committed:  committed,
	}
	return sr, nil
}

// parseContentTypes splits a comma-separated content types string into a
// set for filtering. Returns nil if empty (meaning "allow all").
func parseContentTypes(contentTypes string) map[string]bool {
	if contentTypes == "" {
		return nil
	}
	allowed := make(map[string]bool)
	for ct := range strings.SplitSeq(contentTypes, ",") {
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
		cat := e.Categorizer.Categorize(domain, file.Path, string(file.ContentType), string(body))
		_ = e.Index.IndexFile(domain, file.Path, string(file.ContentType), string(body), cat)
	}

	now := time.Now()
	e.Config.UpdateLastSync(domain, now)
	if err := e.Config.Save(); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	commitMsg := fmt.Sprintf("sync %s: %d files (filtered)", domain, len(mr.Added))
	committed, err := e.Git.Commit(commitMsg)
	if err != nil {
		mr.Errors = append(mr.Errors, fmt.Sprintf("git commit: %v", err))
	}

	sr := &SyncResult{
		SyncResult: *mr,
		SyncTime:   now,
		Committed:  committed,
	}
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
