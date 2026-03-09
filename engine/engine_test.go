package engine

import (
	"context"
	"strings"
	"testing"
)

func TestNewEngine(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	if eng.RootDir != dir {
		t.Errorf("expected RootDir %s, got %s", dir, eng.RootDir)
	}
	if eng.Config == nil {
		t.Fatal("expected Config to be non-nil")
	}
	if eng.Store == nil {
		t.Fatal("expected Store to be non-nil")
	}
	if eng.Index == nil {
		t.Fatal("expected Index to be non-nil")
	}
}

func TestEngineListEmpty(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	sites, err := eng.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("expected 0 sites, got %d", len(sites))
	}
}

func TestEngineStatusUntracked(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	_, err = eng.Status(context.Background(), "nosuch.com")
	if err == nil {
		t.Fatal("expected error for untracked site")
	}
}

func TestEngineSyncUntracked(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	_, err = eng.Sync(context.Background(), "nosuch.com")
	if err == nil {
		t.Fatal("expected error for untracked site")
	}
}

func TestEngineSearchEmpty(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	sr, err := eng.Search(context.Background(), "anything", "", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(sr.Hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(sr.Hits))
	}
	if sr.Suggestion == "" {
		t.Error("expected suggestion for empty results")
	}
}

func TestEngineSearchFullEmpty(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	sfr, err := eng.SearchFull(context.Background(), "anything", "", "", "")
	if err != nil {
		t.Fatalf("SearchFull: %v", err)
	}
	if sfr.Content != "" {
		t.Errorf("expected empty content, got %q", sfr.Content)
	}
	if sfr.Suggestion == "" {
		t.Error("expected suggestion for empty results")
	}
}

func TestEngineRemoveUntracked(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	err = eng.Remove(context.Background(), "nosuch.com", false)
	if err == nil {
		t.Fatal("expected error for untracked site")
	}
}

func TestEngineListFilesUntracked(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	_, err = eng.ListFiles(context.Background(), "nosuch.com")
	if err == nil {
		t.Fatal("expected error for untracked site")
	}
}

func TestEngineRebuildIndex(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	if err := eng.RebuildIndex(context.Background()); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
}

func TestEngineClose(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Close should not error
	if err := eng.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOutlineHintWithMaxDepth(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	// Create a file with headings at multiple depths
	domain := "test.com"
	if err := eng.Config.AddSite(domain, "https://test.com"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Store.EnsureSiteDir(domain); err != nil {
		t.Fatal(err)
	}
	content := "# Title\n\n## Section A\n\nText.\n\n### Subsection A1\n\nMore text.\n\n## Section B\n\nText.\n\n### Subsection B1\n\nText.\n\n### Subsection B2\n\nText.\n\n"
	// Pad to >5000 chars to trigger hint logic
	for len(content) < 6000 {
		content += "Some padding text to make the file large enough for hint logic.\n"
	}
	if _, err := eng.Store.WriteContent(domain, "/docs/big.md", []byte(content)); err != nil {
		t.Fatal(err)
	}

	// With maxDepth=0 (all levels), should have multiple sections, no misleading hint
	out, err := eng.Outline(context.Background(), domain, "/docs/big.md", 0, 0)
	if err != nil {
		t.Fatalf("Outline(depth=0): %v", err)
	}
	if len(out.Sections) <= 1 {
		t.Fatalf("expected multiple sections, got %d", len(out.Sections))
	}
	if out.Hint != "" {
		t.Errorf("expected no hint for full outline, got %q", out.Hint)
	}

	// With maxDepth=1 (only h1), should show hint about hidden sections
	out, err = eng.Outline(context.Background(), domain, "/docs/big.md", 1, 0)
	if err != nil {
		t.Fatalf("Outline(depth=1): %v", err)
	}
	if len(out.Sections) != 1 {
		t.Errorf("expected 1 section at depth=1, got %d", len(out.Sections))
	}
	if out.Hint == "" {
		t.Error("expected hint about hidden sections when maxDepth filters results")
	}
	if out.Hint == "This file has no sub-headings for section-based reading. Use trove_read with max_lines to preview, or trove_search to find specific content within it." {
		t.Error("hint should NOT say 'no sub-headings' when sub-headings exist but are filtered by maxDepth")
	}
}

// TestEngineListFilesWithIndex verifies that ListFiles returns categories from
// the index, and that tag (SetCategory) works on indexed files.
// This is the scenario that failed in production: files were on disk but
// not in the index, causing tag to error with "no indexed file".
func TestEngineListFilesWithIndex(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	domain := "test.com"
	if err := eng.Config.AddSite(domain, "https://test.com"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Store.EnsureSiteDir(domain); err != nil {
		t.Fatal(err)
	}

	// Write files and index them (simulating what sync does)
	files := []struct {
		path, body, cat string
	}{
		{"/docs/api.md", "# API\n```go\nfunc Foo(){}\n```\n", "api-reference"},
		{"/examples/tutorial.md", "# Tutorial\nStep 1: do this.\n", "tutorial"},
		{"/deploy", "# Deploy Guide\nHow to deploy.\n", "guide"},
	}
	for _, f := range files {
		if _, err := eng.Store.WriteContent(domain, f.path, []byte(f.body)); err != nil {
			t.Fatal(err)
		}
		if err := eng.Index.IndexFile(domain, f.path, "companion", f.body, f.cat); err != nil {
			t.Fatal(err)
		}
	}

	// ListFiles should return all 3 with correct categories
	entries, err := eng.ListFiles(context.Background(), domain)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 files, got %d", len(entries))
	}

	catMap := map[string]string{}
	for _, e := range entries {
		catMap[e.Path] = e.Category
	}
	if catMap["/docs/api.md"] != "api-reference" {
		t.Errorf("api.md category = %q, want api-reference", catMap["/docs/api.md"])
	}
	if catMap["/examples/tutorial.md"] != "tutorial" {
		t.Errorf("tutorial.md category = %q, want tutorial", catMap["/examples/tutorial.md"])
	}

	// Tag should work on indexed files
	if err := eng.Tag(context.Background(), domain, "/docs/api.md", "guide"); err != nil {
		t.Fatalf("Tag: %v", err)
	}
	cat, _ := eng.Index.GetCategory(domain, "/docs/api.md")
	if cat != "guide" {
		t.Errorf("after tag: category = %q, want guide", cat)
	}
}

// TestEngineTagMissingFile verifies that tagging a file not in the index
// returns a clear error — the exact scenario that confused users.
func TestEngineTagMissingFile(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	domain := "test.com"
	if err := eng.Config.AddSite(domain, "https://test.com"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Store.EnsureSiteDir(domain); err != nil {
		t.Fatal(err)
	}

	// Write file to disk but DON'T index it — simulates the bug
	if _, err := eng.Store.WriteContent(domain, "/deploy", []byte("# Deploy")); err != nil {
		t.Fatal(err)
	}

	// Tag should fail with clear error
	err = eng.Tag(context.Background(), domain, "/deploy", "guide")
	if err == nil {
		t.Fatal("expected error tagging a file not in the index")
	}
}

// TestEngineSearchPagination verifies the offset/limit/has_more fields.
func TestEngineSearchPagination(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	domain := "test.com"
	if err := eng.Config.AddSite(domain, "https://test.com"); err != nil {
		t.Fatal(err)
	}

	// Index several files all matching "authentication"
	for i := range 5 {
		path := "/docs/" + string(rune('a'+i)) + ".md"
		if err := eng.Index.IndexFile(domain, path, "companion", "authentication docs page", "api-reference"); err != nil {
			t.Fatal(err)
		}
	}

	// Search with limit=2
	sr, err := eng.Search(context.Background(), "authentication", "", "", "", "", 2, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if sr.Limit != 2 {
		t.Errorf("Limit = %d, want 2", sr.Limit)
	}
	if sr.Offset != 0 {
		t.Errorf("Offset = %d, want 0", sr.Offset)
	}
	if !sr.HasMore {
		t.Error("expected HasMore=true with 5 results and limit=2")
	}
	if sr.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", sr.TotalCount)
	}

	// Page 2
	sr, err = eng.Search(context.Background(), "authentication", "", "", "", "", 2, 2)
	if err != nil {
		t.Fatalf("Search page 2: %v", err)
	}
	if sr.Offset != 2 {
		t.Errorf("Offset = %d, want 2", sr.Offset)
	}
	if !sr.HasMore {
		t.Error("expected HasMore=true (offset 2 + 2 results < 5)")
	}

	// Last page
	sr, err = eng.Search(context.Background(), "authentication", "", "", "", "", 2, 4)
	if err != nil {
		t.Fatalf("Search last page: %v", err)
	}
	if sr.HasMore {
		t.Error("expected HasMore=false on last page")
	}
}

// TestEngineListFilesAfterPromotion verifies that ListFiles correctly handles
// promoted files (file→dir/_index) and that the promoted file is readable.
func TestEngineListFilesAfterPromotion(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	domain := "test.com"
	if err := eng.Config.AddSite(domain, "https://test.com"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Store.EnsureSiteDir(domain); err != nil {
		t.Fatal(err)
	}

	// Simulate the deno.com scenario
	if _, err := eng.Store.WriteContent(domain, "/deploy", []byte("# Deploy")); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Store.WriteContent(domain, "/deploy/getting_started", []byte("# Getting Started")); err != nil {
		t.Fatal(err)
	}

	entries, err := eng.ListFiles(context.Background(), domain)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 files, got %d", len(entries))
	}

	// Both should be readable via ReadSection (which uses Store.ReadContent)
	text, err := eng.ReadSection(context.Background(), domain, "/deploy", "", 0)
	if err != nil {
		t.Fatalf("ReadSection /deploy: %v", err)
	}
	if text != "# Deploy" {
		t.Errorf("deploy content = %q", text)
	}

	text, err = eng.ReadSection(context.Background(), domain, "/deploy/getting_started", "", 0)
	if err != nil {
		t.Fatalf("ReadSection /deploy/getting_started: %v", err)
	}
	if text != "# Getting Started" {
		t.Errorf("getting_started content = %q", text)
	}
}

// TestListFilesContentType verifies that ListFiles returns the content_type
// stored in the index, not the one re-derived from the path. This catches the
// bug where Context7 content (content_type="context7") was reported as
// "companion" because ClassifyPath only looks at the filename.
func TestListFilesContentType(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	domain := "context7.com~react"
	if err := eng.Config.AddSite(domain, "react"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Store.EnsureSiteDir(domain); err != nil {
		t.Fatal(err)
	}

	// Simulate what C7 sync does: write a /docs.md file indexed as "context7"
	if _, err := eng.Store.WriteContent(domain, "/docs.md", []byte("# React Docs\nSome content.")); err != nil {
		t.Fatal(err)
	}
	if err := eng.Index.IndexFile(domain, "/docs.md", "context7", "# React Docs\nSome content.", "context7"); err != nil {
		t.Fatal(err)
	}

	entries, err := eng.ListFiles(context.Background(), domain)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	if entries[0].ContentType != "context7" {
		t.Errorf("ContentType = %q, want %q (should come from index, not ClassifyPath)", entries[0].ContentType, "context7")
	}
	if entries[0].Category != "context7" {
		t.Errorf("Category = %q, want %q", entries[0].Category, "context7")
	}
}

// TestSearchPaginationNoSuggestion verifies that paginating past all results
// does NOT produce a misleading "No local results" suggestion.
func TestSearchPaginationNoSuggestion(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	domain := "test.com"
	if err := eng.Config.AddSite(domain, "https://test.com"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Index.IndexFile(domain, "/docs/api.md", "companion", "authentication docs", "api-reference"); err != nil {
		t.Fatal(err)
	}

	// Search with offset past results
	sr, err := eng.Search(context.Background(), "authentication", "", "", "", "", 10, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if sr.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", sr.TotalCount)
	}
	if sr.Suggestion != "" {
		t.Errorf("Suggestion should be empty when results exist but offset is past them, got %q", sr.Suggestion)
	}
}

// TestSearchSuggestionForUntrackedSite verifies the suggestion mentions the
// site is not tracked when filtering to a site that doesn't exist.
func TestSearchSuggestionForUntrackedSite(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	sr, err := eng.Search(context.Background(), "anything", "untracked.com", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if sr.Suggestion == "" {
		t.Error("expected suggestion for untracked site")
	}
	if !strings.Contains(sr.Suggestion, "not tracked") {
		t.Errorf("suggestion should mention site is not tracked, got %q", sr.Suggestion)
	}
}

func TestEngineDefaultProviders(t *testing.T) {
	dir := t.TempDir()
	eng, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = eng.Close() }()

	providers := eng.Discovery.Providers()
	if len(providers) < 1 {
		t.Fatal("expected at least 1 default provider")
	}
	if providers[0].Name() != "site" {
		t.Errorf("expected first provider to be 'site', got %s", providers[0].Name())
	}
}
