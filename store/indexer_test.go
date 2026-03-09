package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexImplementsIndexer(t *testing.T) {
	var _ Indexer = (*Index)(nil)
}

func TestIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Index a file
	if err := idx.IndexFile("example.com", "/llms.txt", "llms-txt", "This is documentation about payments and billing."); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}

	// Search for it
	hits, err := idx.Search("payments", SearchOpts{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", hits[0].Domain)
	}

	// Search with site filter
	hits, err = idx.Search("payments", SearchOpts{Site: "other.com", Limit: 10})
	if err != nil {
		t.Fatalf("Search with filter: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits with wrong site filter, got %d", len(hits))
	}

	// Delete site
	if err := idx.DeleteSite("example.com"); err != nil {
		t.Fatalf("DeleteSite: %v", err)
	}
	hits, err = idx.Search("payments", SearchOpts{Limit: 10})
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits after delete, got %d", len(hits))
	}
}

func TestIndexSetCategoryMissing(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// SetCategory on a file that doesn't exist should error
	err = idx.SetCategory("nosuch.com", "/missing", "guide")
	if err == nil {
		t.Fatal("expected error when setting category on non-existent file")
	}
	if !strings.Contains(err.Error(), "no indexed file") {
		t.Errorf("expected 'no indexed file' error, got %q", err.Error())
	}
}

func TestIndexSetCategoryPersists(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Index a file with auto category
	if err := idx.IndexFile("example.com", "/docs/api.md", "companion", "# API\nEndpoint docs", "api-reference"); err != nil {
		t.Fatal(err)
	}

	// Override category
	if err := idx.SetCategory("example.com", "/docs/api.md", "tutorial"); err != nil {
		t.Fatalf("SetCategory: %v", err)
	}
	cat, _ := idx.GetCategory("example.com", "/docs/api.md")
	if cat != "tutorial" {
		t.Errorf("category = %q, want tutorial", cat)
	}

	// Re-index should preserve user override (user_category=1)
	if err := idx.IndexFile("example.com", "/docs/api.md", "companion", "# API\nEndpoint docs updated", "api-reference"); err != nil {
		t.Fatal(err)
	}
	cat, _ = idx.GetCategory("example.com", "/docs/api.md")
	if cat != "tutorial" {
		t.Errorf("category after re-index = %q, want tutorial (user override should persist)", cat)
	}
}

func TestIndexSetSummaryMissing(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	err = idx.SetSummary("nosuch.com", "/missing", "a summary")
	if err == nil {
		t.Fatal("expected error when setting summary on non-existent file")
	}
}

func TestIndexSetSummarySearchable(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.IndexFile("example.com", "/docs/api.md", "companion", "# API Reference\nBasic endpoint info."); err != nil {
		t.Fatal(err)
	}
	if err := idx.SetSummary("example.com", "/docs/api.md", "Covers webhook verification and payment intents."); err != nil {
		t.Fatal(err)
	}

	// The summary should be searchable via FTS
	hits, err := idx.Search("webhook verification", SearchOpts{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit (via summary), got %d", len(hits))
	}
	if hits[0].Summary != "Covers webhook verification and payment intents." {
		t.Errorf("summary not returned in hit: %q", hits[0].Summary)
	}
}

func TestIndexCategoryCounts(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.IndexFile("example.com", "/api.md", "companion", "api content", "api-reference"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile("example.com", "/guide.md", "companion", "guide content", "guide"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile("example.com", "/tutorial.md", "companion", "tutorial content", "tutorial"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile("example.com", "/api2.md", "companion", "more api", "api-reference"); err != nil {
		t.Fatal(err)
	}

	counts, err := idx.CategoryCounts("example.com")
	if err != nil {
		t.Fatalf("CategoryCounts: %v", err)
	}
	if counts["api-reference"] != 2 {
		t.Errorf("api-reference count = %d, want 2", counts["api-reference"])
	}
	if counts["guide"] != 1 {
		t.Errorf("guide count = %d, want 1", counts["guide"])
	}
	if counts["tutorial"] != 1 {
		t.Errorf("tutorial count = %d, want 1", counts["tutorial"])
	}
}

func TestIndexCacheHeaders(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Before indexing, cache headers should return empty without error
	etag, lm, err := idx.GetCacheHeaders("example.com", "/docs.md")
	if err != nil {
		t.Fatalf("GetCacheHeaders before index: %v", err)
	}
	if etag != "" || lm != "" {
		t.Error("expected empty cache headers for non-existent file")
	}

	// Index a file, then set cache headers
	if err := idx.IndexFile("example.com", "/docs.md", "companion", "content", "other"); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpdateCacheHeaders("example.com", "/docs.md", "etag123", "Mon, 01 Jan 2024"); err != nil {
		t.Fatalf("UpdateCacheHeaders: %v", err)
	}

	etag, lm, _ = idx.GetCacheHeaders("example.com", "/docs.md")
	if etag != "etag123" {
		t.Errorf("etag = %q, want etag123", etag)
	}
	if lm != "Mon, 01 Jan 2024" {
		t.Errorf("last_modified = %q", lm)
	}
}

func TestIndexSearchCategoryFilter(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.IndexFile("example.com", "/api.md", "companion", "authentication docs for API", "api-reference"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile("example.com", "/tutorial.md", "companion", "authentication tutorial step by step", "tutorial"); err != nil {
		t.Fatal(err)
	}

	// Search with category filter
	hits, _ := idx.Search("authentication", SearchOpts{Category: "tutorial", Limit: 10})
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit with tutorial filter, got %d", len(hits))
	}
	if hits[0].Path != "/tutorial.md" {
		t.Errorf("expected tutorial.md, got %s", hits[0].Path)
	}

	// Search with api-reference filter
	hits, _ = idx.Search("authentication", SearchOpts{Category: "api-reference", Limit: 10})
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit with api-reference filter, got %d", len(hits))
	}
	if hits[0].Path != "/api.md" {
		t.Errorf("expected api.md, got %s", hits[0].Path)
	}
}

func TestIndexClassifyPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/llms.txt", "llms-txt"},
		{"/llms-full.txt", "llms-full-txt"},
		{"/llms-ctx.txt", "llms-ctx-txt"},
		{"/llms-ctx-full.txt", "llms-ctx-full-txt"},
		{"/ai.txt", "ai-txt"},
		{"/.well-known/tdmrep.json", "tdmrep"},
		{"/.well-known/agent.json", "well-known"},
		{"/docs/api.md", "companion"},
		{"/anything/else", "companion"},
	}
	for _, tt := range tests {
		got := ClassifyPath(tt.path)
		if got != tt.want {
			t.Errorf("ClassifyPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIndexRebuild(t *testing.T) {
	dir := t.TempDir()

	// Set up store with a file
	s := New(dir)
	if err := s.EnsureSiteDir("test.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}
	siteDir := filepath.Join(dir, "sites", "test.com")
	if err := os.WriteFile(filepath.Join(siteDir, "llms.txt"), []byte("Test content about authentication"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Open index and rebuild
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.Rebuild(s); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// Should find the content
	hits, err := idx.Search("authentication", SearchOpts{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit after rebuild, got %d", len(hits))
	}
}

func TestGetContentType(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Index a file with content_type "context7"
	if err := idx.IndexFile("example.com", "/docs.md", "context7", "React docs content", "context7"); err != nil {
		t.Fatal(err)
	}

	ct, err := idx.GetContentType("example.com", "/docs.md")
	if err != nil {
		t.Fatalf("GetContentType: %v", err)
	}
	if ct != "context7" {
		t.Errorf("content_type = %q, want %q", ct, "context7")
	}

	// Non-existent file should return empty string, no error
	ct, err = idx.GetContentType("example.com", "/nonexistent")
	if err != nil {
		t.Fatalf("GetContentType(missing): %v", err)
	}
	if ct != "" {
		t.Errorf("expected empty content_type for missing file, got %q", ct)
	}
}

func TestSearchInvalidCategory(t *testing.T) {
	dir := t.TempDir()
	idx, err := OpenIndex(dir)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.IndexFile("example.com", "/api.md", "companion", "authentication docs", "api-reference"); err != nil {
		t.Fatal(err)
	}

	// Search with invalid category should return 0 results (filter is a SQL WHERE)
	hits, err := idx.Search("authentication", SearchOpts{Category: "invalid-category", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits with invalid category, got %d", len(hits))
	}
}
