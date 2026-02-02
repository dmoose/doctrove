package mirror

import (
	"context"
	"fmt"

	"github.com/dmoose/llmshadow/internal/discovery"
	"github.com/dmoose/llmshadow/internal/fetcher"
	"github.com/dmoose/llmshadow/internal/store"
)

// Mirror downloads discovered content and writes it to the store.
type Mirror struct {
	Fetcher *fetcher.Fetcher
	Store   *store.Store
}

// New creates a Mirror.
func New(f *fetcher.Fetcher, s *store.Store) *Mirror {
	return &Mirror{Fetcher: f, Store: s}
}

// SyncResult tracks what happened during a sync.
type SyncResult struct {
	Domain  string   `json:"domain"`
	Added   []string `json:"added,omitempty"`
	Updated []string `json:"updated,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// Sync downloads all files from a discovery result and writes them to the store.
func (m *Mirror) Sync(ctx context.Context, result *discovery.Result) (*SyncResult, error) {
	sr := &SyncResult{Domain: result.Domain}

	if err := m.Store.EnsureSiteDir(result.Domain); err != nil {
		return nil, fmt.Errorf("ensuring site dir: %w", err)
	}

	for _, file := range result.Files {
		resp, err := m.Fetcher.Fetch(ctx, file.URL)
		if err != nil {
			sr.Errors = append(sr.Errors, fmt.Sprintf("%s: %v", file.Path, err))
			continue
		}
		if resp == nil {
			// 404 — file disappeared since discovery
			continue
		}

		content := string(resp.Body)

		// Rewrite internal links to local paths
		content = RewriteLinks(content, result.BaseURL)

		wrote, err := m.Store.WriteContent(result.Domain, file.Path, []byte(content))
		if err != nil {
			sr.Errors = append(sr.Errors, fmt.Sprintf("%s: %v", file.Path, err))
			continue
		}
		_ = wrote
		sr.Added = append(sr.Added, file.Path)
	}

	// Write discovery metadata
	if err := m.Store.WriteMeta(result.Domain, "discovered.json", result); err != nil {
		sr.Errors = append(sr.Errors, fmt.Sprintf("metadata: %v", err))
	}

	return sr, nil
}
