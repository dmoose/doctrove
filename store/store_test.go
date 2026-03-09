package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteContentBasic(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.EnsureSiteDir("example.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}

	_, err := s.WriteContent("example.com", "/docs/api.md", []byte("hello"))
	if err != nil {
		t.Fatalf("WriteContent: %v", err)
	}

	got, err := s.ReadContent("example.com", "/docs/api.md")
	if err != nil {
		t.Fatalf("ReadContent: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", string(got), "hello")
	}
}

func TestWriteContentPathCollision(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.EnsureSiteDir("example.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}

	// Write /deploy as a file first
	_, err := s.WriteContent("example.com", "/deploy", []byte("deploy page"))
	if err != nil {
		t.Fatalf("WriteContent /deploy: %v", err)
	}

	// Now write /deploy/getting_started — should promote /deploy to dir
	_, err = s.WriteContent("example.com", "/deploy/getting_started", []byte("getting started"))
	if err != nil {
		t.Fatalf("WriteContent /deploy/getting_started: %v", err)
	}

	// The original /deploy content should be readable via _index fallback
	got, err := s.ReadContent("example.com", "/deploy")
	if err != nil {
		t.Fatalf("ReadContent /deploy after promotion: %v", err)
	}
	if string(got) != "deploy page" {
		t.Errorf("promoted file: got %q, want %q", string(got), "deploy page")
	}

	// The child file should also be readable
	got, err = s.ReadContent("example.com", "/deploy/getting_started")
	if err != nil {
		t.Fatalf("ReadContent /deploy/getting_started: %v", err)
	}
	if string(got) != "getting started" {
		t.Errorf("child file: got %q, want %q", string(got), "getting started")
	}

	// The filesystem should have deploy/ as a directory with _index inside
	deployPath := filepath.Join(s.SiteDir("example.com"), "deploy")
	fi, err := os.Stat(deployPath)
	if err != nil {
		t.Fatalf("Stat deploy: %v", err)
	}
	if !fi.IsDir() {
		t.Error("expected deploy to be a directory after promotion")
	}
	indexPath := filepath.Join(deployPath, "_index")
	if _, err := os.Stat(indexPath); err != nil {
		t.Errorf("expected _index file inside promoted dir: %v", err)
	}
}

func TestWriteContentDeepPathCollision(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.EnsureSiteDir("example.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}

	// Write /a/b as a file
	_, err := s.WriteContent("example.com", "/a/b", []byte("b content"))
	if err != nil {
		t.Fatalf("WriteContent /a/b: %v", err)
	}

	// Write /a/b/c/d — two levels deep past the collision
	_, err = s.WriteContent("example.com", "/a/b/c/d", []byte("d content"))
	if err != nil {
		t.Fatalf("WriteContent /a/b/c/d: %v", err)
	}

	// Both should be readable
	got, err := s.ReadContent("example.com", "/a/b")
	if err != nil {
		t.Fatalf("ReadContent /a/b: %v", err)
	}
	if string(got) != "b content" {
		t.Errorf("got %q, want %q", string(got), "b content")
	}

	got, err = s.ReadContent("example.com", "/a/b/c/d")
	if err != nil {
		t.Fatalf("ReadContent /a/b/c/d: %v", err)
	}
	if string(got) != "d content" {
		t.Errorf("got %q, want %q", string(got), "d content")
	}
}

func TestWriteContentDirectoryExists(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.EnsureSiteDir("example.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}

	// Write /deploy/child first (creates deploy as a directory)
	_, err := s.WriteContent("example.com", "/deploy/child", []byte("child"))
	if err != nil {
		t.Fatalf("WriteContent /deploy/child: %v", err)
	}

	// Now write /deploy itself — should go to deploy/_index
	_, err = s.WriteContent("example.com", "/deploy", []byte("deploy page"))
	if err != nil {
		t.Fatalf("WriteContent /deploy: %v", err)
	}

	// Both should be readable
	got, err := s.ReadContent("example.com", "/deploy")
	if err != nil {
		t.Fatalf("ReadContent /deploy: %v", err)
	}
	if string(got) != "deploy page" {
		t.Errorf("got %q, want %q", string(got), "deploy page")
	}

	got, err = s.ReadContent("example.com", "/deploy/child")
	if err != nil {
		t.Fatalf("ReadContent /deploy/child: %v", err)
	}
	if string(got) != "child" {
		t.Errorf("got %q, want %q", string(got), "child")
	}
}

func TestWriteContentTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.EnsureSiteDir("example.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}

	// Write /deploy/ (trailing slash) then /deploy/child
	_, err := s.WriteContent("example.com", "/deploy/", []byte("index page"))
	if err != nil {
		t.Fatalf("WriteContent /deploy/: %v", err)
	}

	_, err = s.WriteContent("example.com", "/deploy/child", []byte("child"))
	if err != nil {
		t.Fatalf("WriteContent /deploy/child: %v", err)
	}

	// Both should be readable
	got, err := s.ReadContent("example.com", "/deploy/")
	if err != nil {
		t.Fatalf("ReadContent /deploy/: %v", err)
	}
	if string(got) != "index page" {
		t.Errorf("got %q, want %q", string(got), "index page")
	}
}

func TestSiteFileCountAfterPromotion(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.EnsureSiteDir("example.com"); err != nil {
		t.Fatalf("EnsureSiteDir: %v", err)
	}

	if _, err := s.WriteContent("example.com", "/deploy", []byte("page")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteContent("example.com", "/deploy/child", []byte("child")); err != nil {
		t.Fatal(err)
	}

	count, err := s.SiteFileCount("example.com")
	if err != nil {
		t.Fatalf("SiteFileCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files, got %d", count)
	}
}
