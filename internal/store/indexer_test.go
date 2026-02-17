package store

import (
	"os"
	"path/filepath"
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
