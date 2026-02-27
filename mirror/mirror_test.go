package mirror

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dmoose/doctrove/discovery"
	"github.com/dmoose/doctrove/fetcher"
	"github.com/dmoose/doctrove/store"
)

// stubIndexer satisfies store.Indexer for mirror tests without touching SQLite.
type stubIndexer struct{}

func (s *stubIndexer) IndexFile(domain, path, contentType, body string, category ...string) error {
	return nil
}
func (s *stubIndexer) Search(query string, opts store.SearchOpts) ([]store.SearchHit, error) {
	return nil, nil
}
func (s *stubIndexer) DeleteSite(domain string) error                         { return nil }
func (s *stubIndexer) Rebuild(st *store.Store) error                          { return nil }
func (s *stubIndexer) GetCacheHeaders(domain, path string) (string, string, error) {
	return "", "", nil
}
func (s *stubIndexer) UpdateCacheHeaders(domain, path, etag, lastModified string) error {
	return nil
}
func (s *stubIndexer) GetCategory(domain, path string) (string, error)              { return "", nil }
func (s *stubIndexer) SetCategory(domain, path, category string) error              { return nil }
func (s *stubIndexer) GetSummary(domain, path string) (string, string, error)       { return "", "", nil }
func (s *stubIndexer) SetSummary(domain, path, summary string) error                { return nil }
func (s *stubIndexer) CategoryCounts(domain string) (map[string]int, error)         { return nil, nil }
func (s *stubIndexer) Close() error                                                 { return nil }

func TestSyncWithBody(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	f := fetcher.New(fetcher.Options{})
	m := New(f, s, &stubIndexer{})

	result := &discovery.Result{
		Domain:  "test.com",
		BaseURL: "https://test.com",
		Files: []discovery.DiscoveredFile{
			{
				URL:         "https://test.com/docs/api.md",
				Path:        "/docs/api.md",
				ContentType: discovery.TypeCompanion,
				Body:        []byte("# API Documentation\n\nThis is pre-fetched content."),
				Size:        50,
				FoundVia:    "context7",
			},
		},
	}

	sr, err := m.Sync(context.Background(), result, nil)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(sr.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(sr.Added))
	}

	// Verify file was written
	content, err := os.ReadFile(filepath.Join(dir, "sites", "test.com", "docs", "api.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "# API Documentation\n\nThis is pre-fetched content." {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestSyncWithFilter(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	f := fetcher.New(fetcher.Options{})
	m := New(f, s, &stubIndexer{})

	result := &discovery.Result{
		Domain:  "test.com",
		BaseURL: "https://test.com",
		Files: []discovery.DiscoveredFile{
			{
				Path:        "/docs/included.md",
				ContentType: discovery.TypeCompanion,
				Body:        []byte("included"),
			},
			{
				Path:        "/internal/excluded.md",
				ContentType: discovery.TypeCompanion,
				Body:        []byte("excluded"),
			},
		},
	}

	filter := func(path string) bool {
		return path != "/internal/excluded.md"
	}

	sr, err := m.Sync(context.Background(), result, filter)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(sr.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(sr.Added))
	}
	if len(sr.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(sr.Skipped))
	}
}

func TestBuildFilterExclude(t *testing.T) {
	f := BuildFilter(nil, []string{"/internal/*"})
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if !f("/docs/api.md") {
		t.Error("expected /docs/api.md to pass")
	}
	if f("/internal/secret.md") {
		t.Error("expected /internal/secret.md to be excluded")
	}
}

func TestBuildFilterInclude(t *testing.T) {
	f := BuildFilter([]string{"/docs/*"}, nil)
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if !f("/docs/api.md") {
		t.Error("expected /docs/api.md to pass")
	}
	if f("/other/page.md") {
		t.Error("expected /other/page.md to be excluded")
	}
}

func TestBuildFilterIncludeAndExclude(t *testing.T) {
	f := BuildFilter([]string{"/docs/*"}, []string{"/docs/internal.md"})
	if !f("/docs/api.md") {
		t.Error("expected /docs/api.md to pass")
	}
	if f("/docs/internal.md") {
		t.Error("expected /docs/internal.md to be excluded (exclude takes precedence)")
	}
}

func TestBuildFilterNil(t *testing.T) {
	f := BuildFilter(nil, nil)
	if f != nil {
		t.Error("expected nil filter when no include/exclude")
	}
}

func TestRewriteLinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		baseURL string
		want    string
	}{
		{
			name:    "absolute to relative",
			content: "See [API](https://example.com/docs/api) for details.",
			baseURL: "https://example.com",
			want:    "See [API](./docs/api) for details.",
		},
		{
			name:    "no match",
			content: "See [API](https://other.com/docs/api) for details.",
			baseURL: "https://example.com",
			want:    "See [API](https://other.com/docs/api) for details.",
		},
		{
			name:    "trailing slash on base",
			content: "Link: https://example.com/page",
			baseURL: "https://example.com/",
			want:    "Link: ./page",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RewriteLinks(tt.content, tt.baseURL)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSyncMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	f := fetcher.New(fetcher.Options{})
	m := New(f, s, &stubIndexer{})

	result := &discovery.Result{
		Domain:  "multi.com",
		BaseURL: "https://multi.com",
		Files: []discovery.DiscoveredFile{
			{Path: "/llms.txt", ContentType: discovery.TypeLLMSTxt, Body: []byte("# LLMs.txt")},
			{Path: "/docs/a.md", ContentType: discovery.TypeCompanion, Body: []byte("# Doc A")},
			{Path: "/docs/b.md", ContentType: discovery.TypeCompanion, Body: []byte("# Doc B")},
		},
	}

	sr, err := m.Sync(context.Background(), result, nil)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(sr.Added) != 3 {
		t.Errorf("expected 3 added, got %d", len(sr.Added))
	}
	if sr.Domain != "multi.com" {
		t.Errorf("expected domain multi.com, got %s", sr.Domain)
	}
}

func TestSyncPathCollision(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	f := fetcher.New(fetcher.Options{})
	m := New(f, s, &stubIndexer{})

	// Simulate the deno.com scenario: /deploy as a page, then /deploy/getting_started
	result := &discovery.Result{
		Domain:  "docs.deno.com",
		BaseURL: "https://docs.deno.com",
		Files: []discovery.DiscoveredFile{
			{Path: "/deploy", ContentType: discovery.TypeCompanion, Body: []byte("# Deno Deploy")},
			{Path: "/deploy/getting_started", ContentType: discovery.TypeCompanion, Body: []byte("# Getting Started")},
			{Path: "/deploy/kv", ContentType: discovery.TypeCompanion, Body: []byte("# KV Store")},
		},
	}

	sr, err := m.Sync(context.Background(), result, nil)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(sr.Added) != 3 {
		t.Errorf("expected 3 added, got %d (errors: %v)", len(sr.Added), sr.Errors)
	}
	if len(sr.Errors) != 0 {
		t.Errorf("expected 0 errors, got %v", sr.Errors)
	}

	// All three files should be readable
	for _, path := range []string{"/deploy", "/deploy/getting_started", "/deploy/kv"} {
		if _, err := s.ReadContent("docs.deno.com", path); err != nil {
			t.Errorf("ReadContent(%q): %v", path, err)
		}
	}
}

func TestSyncEmptyResult(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	f := fetcher.New(fetcher.Options{})
	m := New(f, s, &stubIndexer{})

	result := &discovery.Result{
		Domain:  "empty.com",
		BaseURL: "https://empty.com",
	}

	sr, err := m.Sync(context.Background(), result, nil)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(sr.Added) != 0 {
		t.Errorf("expected 0 added, got %d", len(sr.Added))
	}
}
