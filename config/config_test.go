package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s.RateLimit != 2 {
		t.Errorf("RateLimit = %d, want 2", s.RateLimit)
	}
	if s.RateBurst != 5 {
		t.Errorf("RateBurst = %d, want 5", s.RateBurst)
	}
	if s.Timeout != "30s" {
		t.Errorf("Timeout = %q, want 30s", s.Timeout)
	}
	if s.MaxProbes != 100 {
		t.Errorf("MaxProbes = %d, want 100", s.MaxProbes)
	}
	if s.UserAgent != "doctrove/0.1" {
		t.Errorf("UserAgent = %q, want doctrove/0.1", s.UserAgent)
	}
}

func TestTimeoutDuration(t *testing.T) {
	s := DefaultSettings()
	d := s.TimeoutDuration()
	if d.Seconds() != 30 {
		t.Errorf("TimeoutDuration = %v, want 30s", d)
	}

	s.Timeout = "invalid"
	d = s.TimeoutDuration()
	if d.Seconds() != 30 {
		t.Errorf("invalid timeout should fallback to 30s, got %v", d)
	}
}

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Settings == nil {
		t.Fatal("expected default settings")
	}
	if cfg.Settings.RateLimit != 2 {
		t.Errorf("expected default RateLimit=2, got %d", cfg.Settings.RateLimit)
	}
	if len(cfg.Sites) != 0 {
		t.Errorf("expected 0 sites, got %d", len(cfg.Sites))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	cfg.AddSite("example.com", "https://example.com")
	cfg.Settings.Context7APIKey = "ctx7sk-test123"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload
	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if _, ok := cfg2.Sites["example.com"]; !ok {
		t.Error("expected example.com in reloaded config")
	}
	if cfg2.Settings.Context7APIKey != "ctx7sk-test123" {
		t.Errorf("C7 key = %q, want ctx7sk-test123", cfg2.Settings.Context7APIKey)
	}
}

func TestContext7KeyAppearsInYAML(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	// Key is empty — but should still appear in YAML (no omitempty)
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, DefaultConfigFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "context7_api_key") {
		t.Error("context7_api_key should appear in YAML even when empty")
	}
}

func TestAddSiteDuplicate(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	if err := cfg.AddSite("example.com", "https://example.com"); err != nil {
		t.Fatalf("first AddSite: %v", err)
	}
	if err := cfg.AddSite("example.com", "https://example.com"); err == nil {
		t.Error("expected error for duplicate site")
	}
}

func TestRemoveSite(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	cfg.AddSite("example.com", "https://example.com")
	if err := cfg.RemoveSite("example.com"); err != nil {
		t.Fatalf("RemoveSite: %v", err)
	}
	if _, ok := cfg.Sites["example.com"]; ok {
		t.Error("site should be removed")
	}
}

func TestRemoveSiteMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	if err := cfg.RemoveSite("nosuch.com"); err == nil {
		t.Error("expected error removing non-existent site")
	}
}

func TestMergeSettingsPartial(t *testing.T) {
	dir := t.TempDir()
	// Write partial config — only set rate_limit
	yaml := "settings:\n  rate_limit: 10\n"
	os.WriteFile(filepath.Join(dir, DefaultConfigFile), []byte(yaml), 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Settings.RateLimit != 10 {
		t.Errorf("RateLimit = %d, want 10 (from file)", cfg.Settings.RateLimit)
	}
	if cfg.Settings.RateBurst != 5 {
		t.Errorf("RateBurst = %d, want 5 (from default)", cfg.Settings.RateBurst)
	}
	if cfg.Settings.Timeout != "30s" {
		t.Errorf("Timeout = %q, want 30s (from default)", cfg.Settings.Timeout)
	}
}

func TestUpdateLastSync(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	cfg.AddSite("example.com", "https://example.com")

	now := cfg.Sites["example.com"].LastSync
	if !now.IsZero() {
		t.Error("expected zero LastSync initially")
	}

	cfg.UpdateLastSync("example.com", now)
}

func TestSetContentTypes(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	cfg.AddSite("example.com", "https://example.com")
	cfg.SetContentTypes("example.com", "llms-txt,llms-full-txt")
	if cfg.Sites["example.com"].ContentTypes != "llms-txt,llms-full-txt" {
		t.Errorf("ContentTypes = %q", cfg.Sites["example.com"].ContentTypes)
	}
}
