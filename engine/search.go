package engine

import (
	"context"
	"fmt"

	"github.com/dmoose/doctrove/store"
)

// SearchHit re-exports store.SearchHit.
type SearchHit = store.SearchHit

// SearchResult wraps search hits with metadata.
type SearchResult struct {
	Hits       []SearchHit `json:"results"`
	TotalCount int         `json:"total_count"`
	Offset     int         `json:"offset,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	HasMore    bool        `json:"has_more,omitempty"`
	Suggestion string      `json:"suggestion,omitempty"`
}

// SearchFullResult contains the full content of the best search match.
type SearchFullResult struct {
	Domain      string `json:"domain,omitempty"`
	Path        string `json:"path,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Category    string `json:"category,omitempty"`
	Size        int    `json:"size,omitempty"`
	Content     string `json:"content,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// Search performs a full-text search across indexed content.
func (e *Engine) Search(ctx context.Context, query string, site, contentType, category, path string, limit, offset int) (*SearchResult, error) {
	hits, err := e.Index.Search(query, store.SearchOpts{
		Site:        site,
		ContentType: contentType,
		Category:    category,
		Path:        path,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}

	// Get total count (same filters, no limit/offset)
	totalCount := len(hits)
	if offset > 0 || len(hits) == limit {
		// Only run count query when pagination might be hiding results
		allHits, countErr := e.Index.Search(query, store.SearchOpts{
			Site:        site,
			ContentType: contentType,
			Category:    category,
			Path:        path,
			Limit:       10000,
			Offset:      0,
		})
		if countErr == nil {
			totalCount = len(allHits)
		}
	}

	result := &SearchResult{
		Hits:       hits,
		TotalCount: totalCount,
		Offset:     offset,
		Limit:      limit,
		HasMore:    offset+len(hits) < totalCount,
	}
	if len(hits) == 0 {
		if site != "" {
			if _, ok := e.Config.Sites[site]; !ok {
				result.Suggestion = fmt.Sprintf("Site %q is not tracked. Use trove_scan to add it, or trove_list to see tracked sites.", site)
			} else {
				result.Suggestion = fmt.Sprintf("No results for this query on %s. Try broader terms or check trove_list_files to see what content is available.", site)
			}
		} else {
			result.Suggestion = "No local results. Use trove_discover to check if a URL has LLM content, or trove_scan to add and sync a site."
		}
	}
	return result, nil
}

// SearchFull searches and returns the full content of the top hit.
func (e *Engine) SearchFull(ctx context.Context, query string, site, contentType, category string) (*SearchFullResult, error) {
	sr, err := e.Search(ctx, query, site, contentType, category, "", 1, 0)
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
	result.Size = len(body)
	result.Content = string(body)
	return result, nil
}

// RebuildIndex rebuilds the search index from files on disk.
func (e *Engine) RebuildIndex(ctx context.Context) error {
	return e.Index.Rebuild(e.Store)
}
