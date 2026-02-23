package engine

import (
	"context"
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

	sr, err := eng.Search(context.Background(), "anything", "", "", "", 10, 0)
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
