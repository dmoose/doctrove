package mirror

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dmoose/doctrove/discovery"
	"github.com/dmoose/doctrove/fetcher"
	"github.com/dmoose/doctrove/store"
)

// Mirror downloads discovered content and writes it to the store.
type Mirror struct {
	Fetcher fetcher.HTTPFetcher
	Store   *store.Store
	Index   store.Indexer
}

// New creates a Mirror.
func New(f fetcher.HTTPFetcher, s *store.Store, idx store.Indexer) *Mirror {
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
// Supports ** for recursive directory matching (e.g. "/docs/**/*.md").
func BuildFilter(include, exclude []string) FilterFunc {
	if len(include) == 0 && len(exclude) == 0 {
		return nil // no filtering
	}
	return func(path string) bool {
		// Check excludes first
		for _, pattern := range exclude {
			if matchGlob(pattern, path) {
				return false
			}
		}
		// If no includes specified, everything passes
		if len(include) == 0 {
			return true
		}
		// Must match at least one include
		for _, pattern := range include {
			if matchGlob(pattern, path) {
				return true
			}
		}
		return false
	}
}

// matchGlob matches a path against a glob pattern, supporting ** for recursive
// directory matching. ** matches zero or more path segments.
func matchGlob(pattern, path string) bool {
	// If no **, fall back to filepath.Match
	if !strings.Contains(pattern, "**") {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	// Split on ** and match each segment
	parts := strings.Split(pattern, "**")
	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]
		// prefix must match the start
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}
		// suffix must match the end (using filepath.Match for the tail)
		remaining := path[len(prefix):]
		if suffix == "" || suffix == "/" {
			return true
		}
		suffix = strings.TrimPrefix(suffix, "/")
		// Check if any tail of the remaining path matches the suffix pattern
		segments := strings.Split(remaining, "/")
		for i := range segments {
			tail := strings.Join(segments[i:], "/")
			if matched, _ := filepath.Match(suffix, tail); matched {
				return true
			}
			// Also try matching just the filename for patterns like **/*.md
			if matched, _ := filepath.Match(suffix, segments[len(segments)-1]); matched {
				return true
			}
		}
		return false
	}

	// Fallback for multiple ** — treat as simple contains
	matched, _ := filepath.Match(pattern, path)
	return matched
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
				// Skip JS-only SPA shells — they need browser rendering we don't have
				if fetcher.IsJSHeavy(content) {
					sr.Errors = append(sr.Errors, fmt.Sprintf("%s: skipped (JS-heavy SPA, needs browser rendering)", file.Path))
					continue
				}
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

		// Strip MDX/JSX framework artifacts from markdown content
		content = fetcher.CleanMDX(content)

		// Compare with existing content to classify as Added/Updated/Unchanged
		newBytes := []byte(content)
		existing, readErr := m.Store.ReadContent(result.Domain, file.Path)
		if readErr == nil && bytes.Equal(existing, newBytes) {
			sr.Unchanged = append(sr.Unchanged, file.Path)
			continue
		}

		_, writeErr := m.Store.WriteContent(result.Domain, file.Path, newBytes)
		if writeErr != nil {
			sr.Errors = append(sr.Errors, fmt.Sprintf("%s: %v", file.Path, writeErr))
			continue
		}
		if readErr == nil {
			sr.Updated = append(sr.Updated, file.Path)
		} else {
			sr.Added = append(sr.Added, file.Path)
		}
	}

	if err := m.Store.WriteMeta(result.Domain, "discovered.json", result); err != nil {
		sr.Errors = append(sr.Errors, fmt.Sprintf("metadata: %v", err))
	}

	return sr, nil
}
