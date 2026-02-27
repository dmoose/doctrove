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
	Root string // Root directory of the doctrove workspace
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
//
// If a path component already exists as a regular file (e.g., writing
// "/deploy/getting_started" when "/deploy" is a file), the conflicting file
// is promoted into a directory by renaming it to "<dir>/_index". This matches
// how web servers treat directory-index pages.
func (s *Store) WriteContent(domain, urlPath string, data []byte) (string, error) {
	// Clean the URL path and convert to a filesystem path
	clean := strings.TrimPrefix(urlPath, "/")
	if clean == "" {
		clean = "index.txt"
	}
	dest := filepath.Join(s.SiteDir(domain), clean)

	// If dest itself is an existing directory, write as _index inside it.
	if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
		dest = filepath.Join(dest, "_index")
	}

	// Resolve file/directory conflicts: walk from dest upward and promote
	// any regular file that needs to become a directory.
	if relocated, err := s.resolvePathConflicts(domain, dest); err != nil {
		return "", fmt.Errorf("resolving path conflict: %w", err)
	} else if relocated != "" {
		// re-derive dest in case the conflict was at our own path
		dest = filepath.Join(s.SiteDir(domain), clean)
		if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
			dest = filepath.Join(dest, "_index")
		}
		_ = relocated // tracked by caller via ReadContent fallback
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("creating parent dir: %w", err)
	}

	if err := os.WriteFile(dest, data, 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", dest, err)
	}
	return dest, nil
}

// resolvePathConflicts checks whether any ancestor of dest is a regular file
// instead of a directory. If so, the file is promoted: renamed to <dir>/_index
// so that the directory can be created. Returns the relocated path (or "").
func (s *Store) resolvePathConflicts(domain, dest string) (relocated string, err error) {
	siteDir := s.SiteDir(domain)

	// Collect path components from dest up to siteDir and check top-down.
	// This handles deep conflicts (e.g., /a/b is a file, writing /a/b/c/d)
	// because after promoting /a/b, /a/b/c doesn't need special handling.
	var components []string
	dir := filepath.Dir(dest)
	for dir != siteDir && len(dir) > len(siteDir) {
		components = append(components, dir)
		dir = filepath.Dir(dir)
	}

	// Check top-down (reverse order) so we fix the shallowest conflict first
	for i := len(components) - 1; i >= 0; i-- {
		p := components[i]
		fi, statErr := os.Stat(p)
		if statErr != nil {
			continue // doesn't exist yet or parent doesn't exist — fine
		}
		if fi.IsDir() {
			continue // already a directory — fine
		}
		// p is a regular file — promote to directory with _index
		tmp := p + ".__promote"
		if err := os.Rename(p, tmp); err != nil {
			return "", fmt.Errorf("renaming %s for promotion: %w", p, err)
		}
		if err := os.MkdirAll(p, 0755); err != nil {
			_ = os.Rename(tmp, p) // rollback
			return "", fmt.Errorf("creating dir after promotion: %w", err)
		}
		target := filepath.Join(p, "_index")
		if err := os.Rename(tmp, target); err != nil {
			return "", fmt.Errorf("moving promoted file: %w", err)
		}
		rel, _ := filepath.Rel(siteDir, p)
		return "/" + rel, nil
	}
	return "", nil
}

// ReadContent reads a file from a site's directory by its URL path.
// If the direct path is a directory (promoted via conflict resolution),
// it falls back to reading <path>/_index.
func (s *Store) ReadContent(domain, urlPath string) ([]byte, error) {
	clean := strings.TrimPrefix(urlPath, "/")
	if clean == "" {
		clean = "index.txt"
	}
	full := filepath.Join(s.SiteDir(domain), clean)
	fi, err := os.Stat(full)
	if err == nil && fi.IsDir() {
		full = filepath.Join(full, "_index")
	}
	return os.ReadFile(full)
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
