package mirror

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dmoose/llmshadow/internal/discovery"
	"github.com/dmoose/llmshadow/internal/fetcher"
	"github.com/dmoose/llmshadow/internal/store"
)

// Mirror downloads discovered content and writes it to the store.
type Mirror struct {
	Fetcher *fetcher.Fetcher
	Store   *store.Store
	Index   store.Indexer
}

// New creates a Mirror.
func New(f *fetcher.Fetcher, s *store.Store, idx store.Indexer) *Mirror {
	return &Mirror{Fetcher: f, Store: s, Index: idx}
}

// SyncResult tracks what happened during a sync.
type SyncResult struct {
	Domain    string   `json:"domain"`
	Added     []string `json:"added,omitempty"`
	Updated   []string `json:"updated,omitempty"`
	Unchanged []string `json:"unchanged,omitempty"`
	Skipped   []string `json:"skipped,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

// FilterFunc returns true if a path should be included in the sync.
type FilterFunc func(path string) bool

// BuildFilter creates a FilterFunc from include/exclude glob patterns.
// If include is empty, all paths match. Exclude takes precedence.
func BuildFilter(include, exclude []string) FilterFunc {
	if len(include) == 0 && len(exclude) == 0 {
		return nil // no filtering
	}
	return func(path string) bool {
		// Check excludes first
		for _, pattern := range exclude {
			if matched, _ := filepath.Match(pattern, path); matched {
				return false
			}
		}
		// If no includes specified, everything passes
		if len(include) == 0 {
			return true
		}
		// Must match at least one include
		for _, pattern := range include {
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
		}
		return false
	}
}

// Sync downloads all files from a discovery result and writes them to the store.
// If filter is non-nil, only paths that pass the filter are downloaded.
func (m *Mirror) Sync(ctx context.Context, result *discovery.Result, filter FilterFunc) (*SyncResult, error) {
	sr := &SyncResult{Domain: result.Domain}

	if err := m.Store.EnsureSiteDir(result.Domain); err != nil {
		return nil, fmt.Errorf("ensuring site dir: %w", err)
	}

	for _, file := range result.Files {
		// Apply include/exclude filter
		if filter != nil && !filter(file.Path) {
			sr.Skipped = append(sr.Skipped, file.Path)
			continue
		}

		var content string

		if file.Body != nil {
			// Provider already fetched the content (e.g., Context7, DevDocs)
			content = string(file.Body)
		} else {
			// Try conditional fetch if we have cached headers
			var (
				resp    *fetcher.Response
				fetchErr error
			)
			etag, lastMod, _ := m.Index.GetCacheHeaders(result.Domain, file.Path)
			if etag != "" || lastMod != "" {
				resp, fetchErr = m.Fetcher.FetchConditional(ctx, file.URL, etag, lastMod)
			} else {
				resp, fetchErr = m.Fetcher.Fetch(ctx, file.URL)
			}
			if fetchErr != nil {
				sr.Errors = append(sr.Errors, fmt.Sprintf("%s: %v", file.Path, fetchErr))
				continue
			}
			if resp == nil {
				// 304 Not Modified or 404
				if etag != "" || lastMod != "" {
					sr.Unchanged = append(sr.Unchanged, file.Path)
				}
				continue
			}

			content = string(resp.Body)

			// Convert HTML to markdown if needed
			if fetcher.IsHTML(resp.ContentType, resp.Body) {
				md, convErr := fetcher.ConvertHTML(content)
				if convErr != nil || len(md) < 50 {
					sr.Errors = append(sr.Errors, fmt.Sprintf("%s: rejected (HTML, conversion failed)", file.Path))
					continue
				}
				content = md
			}

			// Store cache headers for next sync
			if resp.ETag != "" || resp.LastModified != "" {
				_ = m.Index.UpdateCacheHeaders(result.Domain, file.Path, resp.ETag, resp.LastModified)
			}
		}
		content = RewriteLinks(content, result.BaseURL)

		_, writeErr := m.Store.WriteContent(result.Domain, file.Path, []byte(content))
		if writeErr != nil {
			sr.Errors = append(sr.Errors, fmt.Sprintf("%s: %v", file.Path, writeErr))
			continue
		}
		sr.Added = append(sr.Added, file.Path)
	}

	if err := m.Store.WriteMeta(result.Domain, "discovered.json", result); err != nil {
		sr.Errors = append(sr.Errors, fmt.Sprintf("metadata: %v", err))
	}

	return sr, nil
}
