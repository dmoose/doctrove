package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Index manages the SQLite FTS5 search index.
type Index struct {
	db   *sql.DB
	path string
}

// OpenIndex opens or creates the search index database.
func OpenIndex(rootDir string) (*Index, error) {
	dbPath := filepath.Join(rootDir, "llmshadow.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening index db: %w", err)
	}

	// Enable WAL mode for concurrent read access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	idx := &Index{db: db, path: dbPath}
	if err := idx.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return idx, nil
}

func (idx *Index) ensureSchema() error {
	_, err := idx.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS content USING fts5(
			domain,
			path,
			content_type,
			body,
			tokenize='porter unicode61'
		);

		CREATE TABLE IF NOT EXISTS content_meta (
			domain TEXT NOT NULL,
			path TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			indexed_at TEXT NOT NULL,
			PRIMARY KEY (domain, path)
		);
	`)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}
	return nil
}

// IndexFile adds or updates a file in the search index.
func (idx *Index) IndexFile(domain, urlPath, contentType, body string) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove old entry from FTS table
	_, err = tx.Exec(
		"DELETE FROM content WHERE domain = ? AND path = ?",
		domain, urlPath,
	)
	if err != nil {
		return fmt.Errorf("deleting old fts entry: %w", err)
	}

	// Insert new entry
	_, err = tx.Exec(
		"INSERT INTO content (domain, path, content_type, body) VALUES (?, ?, ?, ?)",
		domain, urlPath, contentType, body,
	)
	if err != nil {
		return fmt.Errorf("inserting fts entry: %w", err)
	}

	// Upsert metadata
	_, err = tx.Exec(`
		INSERT INTO content_meta (domain, path, content_type, size, indexed_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(domain, path) DO UPDATE SET
			content_type = excluded.content_type,
			size = excluded.size,
			indexed_at = excluded.indexed_at
	`, domain, urlPath, contentType, len(body))
	if err != nil {
		return fmt.Errorf("upserting metadata: %w", err)
	}

	return tx.Commit()
}

// SearchHit represents a single search result.
type SearchHit struct {
	Domain      string  `json:"domain"`
	Path        string  `json:"path"`
	ContentType string  `json:"content_type"`
	Snippet     string  `json:"snippet"`
	Rank        float64 `json:"rank"`
}

// SearchOpts controls search behavior.
type SearchOpts struct {
	Site        string // filter to a specific domain
	ContentType string // filter to a content type
	Limit       int
}

// Search performs a full-text search across all indexed content.
func (idx *Index) Search(query string, opts SearchOpts) ([]SearchHit, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "content MATCH ?")
	args = append(args, query)

	if opts.Site != "" {
		conditions = append(conditions, "domain = ?")
		args = append(args, opts.Site)
	}
	if opts.ContentType != "" {
		conditions = append(conditions, "content_type = ?")
		args = append(args, opts.ContentType)
	}

	where := strings.Join(conditions, " AND ")
	args = append(args, opts.Limit)

	q := fmt.Sprintf(`
		SELECT domain, path, content_type, snippet(content, 3, '>>>', '<<<', '...', 40), rank
		FROM content
		WHERE %s
		ORDER BY rank
		LIMIT ?
	`, where)

	rows, err := idx.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.Domain, &h.Path, &h.ContentType, &h.Snippet, &h.Rank); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// Rebuild drops and recreates the FTS index from files on disk.
func (idx *Index) Rebuild(store *Store) error {
	// Clear existing data
	if _, err := idx.db.Exec("DELETE FROM content"); err != nil {
		return fmt.Errorf("clearing fts: %w", err)
	}
	if _, err := idx.db.Exec("DELETE FROM content_meta"); err != nil {
		return fmt.Errorf("clearing meta: %w", err)
	}

	sites, err := store.ListSites()
	if err != nil {
		return err
	}

	for _, domain := range sites {
		siteDir := store.SiteDir(domain)
		err := filepath.Walk(siteDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "_meta" {
					return filepath.SkipDir
				}
				return nil
			}

			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Derive URL path from filesystem path
			rel, _ := filepath.Rel(siteDir, path)
			urlPath := "/" + rel
			ct := classifyPath(urlPath)

			return idx.IndexFile(domain, urlPath, ct, string(body))
		})
		if err != nil {
			return fmt.Errorf("indexing %s: %w", domain, err)
		}
	}
	return nil
}

// Close closes the database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

func classifyPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, "/llms.txt") || lower == "/llms.txt":
		return "llms-txt"
	case strings.HasSuffix(lower, "/llms-full.txt") || lower == "/llms-full.txt":
		return "llms-full-txt"
	case strings.HasSuffix(lower, "/ai.txt") || lower == "/ai.txt":
		return "ai-txt"
	case strings.Contains(lower, "/.well-known/tdmrep"):
		return "tdmrep"
	case strings.Contains(lower, "/.well-known/"):
		return "well-known"
	default:
		return "companion"
	}
}
