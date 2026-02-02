package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store manages the filesystem layout for mirrored content.
type Store struct {
	Root string // Root directory of the llmshadow workspace
}

// New creates a Store rooted at the given directory.
func New(root string) *Store {
	return &Store{Root: root}
}

// SitesDir returns the path to the sites directory.
func (s *Store) SitesDir() string {
	return filepath.Join(s.Root, "sites")
}

// SiteDir returns the path for a specific site.
func (s *Store) SiteDir(domain string) string {
	return filepath.Join(s.SitesDir(), domain)
}

// MetaDir returns the _meta directory for a site.
func (s *Store) MetaDir(domain string) string {
	return filepath.Join(s.SiteDir(domain), "_meta")
}

// EnsureSiteDir creates the directory structure for a site.
func (s *Store) EnsureSiteDir(domain string) error {
	dirs := []string{
		s.SiteDir(domain),
		s.MetaDir(domain),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}

// WriteContent writes content to the appropriate path under a site's directory.
// urlPath is the path portion of the URL (e.g., "/llms.txt", "/docs/api.html.md").
func (s *Store) WriteContent(domain, urlPath string, data []byte) (string, error) {
	// Clean the URL path and convert to a filesystem path
	clean := strings.TrimPrefix(urlPath, "/")
	if clean == "" {
		clean = "index.txt"
	}
	dest := filepath.Join(s.SiteDir(domain), clean)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("creating parent dir: %w", err)
	}

	if err := os.WriteFile(dest, data, 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", dest, err)
	}
	return dest, nil
}

// WriteMeta writes a JSON metadata file to the site's _meta directory.
func (s *Store) WriteMeta(domain, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	dest := filepath.Join(s.MetaDir(domain), name)
	return os.WriteFile(dest, data, 0644)
}

// ReadMeta reads a JSON metadata file from the site's _meta directory.
func (s *Store) ReadMeta(domain, name string, v any) error {
	data, err := os.ReadFile(filepath.Join(s.MetaDir(domain), name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ListSites returns the domain names of all tracked sites.
func (s *Store) ListSites() ([]string, error) {
	entries, err := os.ReadDir(s.SitesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing sites: %w", err)
	}
	var sites []string
	for _, e := range entries {
		if e.IsDir() {
			sites = append(sites, e.Name())
		}
	}
	return sites, nil
}

// SiteFileCount returns the number of content files (excluding _meta) for a site.
func (s *Store) SiteFileCount(domain string) (int, error) {
	count := 0
	siteDir := s.SiteDir(domain)
	err := filepath.Walk(siteDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip _meta directory
		if info.IsDir() && info.Name() == "_meta" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}
