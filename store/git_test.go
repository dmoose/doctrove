package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitGitFresh(t *testing.T) {
	dir := t.TempDir()
	gs, err := InitGit(dir)
	if err != nil {
		t.Fatalf("InitGit: %v", err)
	}

	// HEAD should resolve after seed commit.
	if _, err := gs.repo.Head(); err != nil {
		t.Fatalf("HEAD not valid after init: %v", err)
	}

	// .gitignore should exist.
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}

	// Log should have exactly one entry (the seed commit).
	entries, err := gs.Log("", 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Message != "init workspace" {
		t.Errorf("seed commit message = %q, want %q", entries[0].Message, "init workspace")
	}
}

func TestInitGitIdempotent(t *testing.T) {
	dir := t.TempDir()

	gs1, err := InitGit(dir)
	if err != nil {
		t.Fatalf("first InitGit: %v", err)
	}

	// Write a file and commit so there's history.
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := gs1.Commit("add test file"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Open again — should not error, should preserve history.
	gs2, err := InitGit(dir)
	if err != nil {
		t.Fatalf("second InitGit: %v", err)
	}
	entries, err := gs2.Log("", 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries after reopen, got %d", len(entries))
	}
}

func TestCommitAfterInit(t *testing.T) {
	dir := t.TempDir()
	gs, err := InitGit(dir)
	if err != nil {
		t.Fatalf("InitGit: %v", err)
	}

	// Write a file and commit.
	sitesDir := filepath.Join(dir, "sites", "example.com")
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sitesDir, "llms.txt"), []byte("# Example"), 0644); err != nil {
		t.Fatal(err)
	}

	committed, err := gs.Commit("sync example.com: 1 files")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if !committed {
		t.Fatal("expected commit to be created")
	}

	entries, err := gs.Log("", 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries (seed + sync), got %d", len(entries))
	}
}

func TestCommitNoChanges(t *testing.T) {
	dir := t.TempDir()
	gs, err := InitGit(dir)
	if err != nil {
		t.Fatalf("InitGit: %v", err)
	}

	committed, err := gs.Commit("should be empty")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if committed {
		t.Fatal("expected no commit when nothing changed")
	}
}

func TestLogWithSiteFilter(t *testing.T) {
	dir := t.TempDir()
	gs, err := InitGit(dir)
	if err != nil {
		t.Fatalf("InitGit: %v", err)
	}

	// Create files for two sites.
	for _, site := range []string{"a.com", "b.com"} {
		d := filepath.Join(dir, "sites", site)
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "llms.txt"), []byte("# "+site), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := gs.Commit("sync " + site); err != nil {
			t.Fatalf("Commit %s: %v", site, err)
		}
	}

	// Filter to a.com — should get 1 entry (not the b.com commit or seed).
	entries, err := gs.Log("a.com", 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry for a.com, got %d", len(entries))
	}
}

func TestDiffBetweenCommits(t *testing.T) {
	dir := t.TempDir()
	gs, err := InitGit(dir)
	if err != nil {
		t.Fatalf("InitGit: %v", err)
	}

	// Create a file and commit.
	if err := os.WriteFile(filepath.Join(dir, "doc.txt"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := gs.Commit("v1"); err != nil {
		t.Fatalf("Commit v1: %v", err)
	}

	// Modify and commit again.
	if err := os.WriteFile(filepath.Join(dir, "doc.txt"), []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := gs.Commit("v2"); err != nil {
		t.Fatalf("Commit v2: %v", err)
	}

	// Diff HEAD~1..HEAD should show the change.
	diff, err := gs.Diff("HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
}

func TestHasChanges(t *testing.T) {
	dir := t.TempDir()
	gs, err := InitGit(dir)
	if err != nil {
		t.Fatalf("InitGit: %v", err)
	}

	// Clean after init.
	has, err := gs.HasChanges()
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if has {
		t.Fatal("expected clean worktree after init")
	}

	// Write a file — now dirty.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	has, err = gs.HasChanges()
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if !has {
		t.Fatal("expected dirty worktree after writing file")
	}
}
