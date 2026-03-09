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
	dbPath := filepath.Join(rootDir, "doctrove.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening index db: %w", err)
	}

	// Enable WAL mode for concurrent read access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	idx := &Index{db: db, path: dbPath}
	if err := idx.ensureSchema(); err != nil {
		_ = db.Close()
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
			category TEXT NOT NULL DEFAULT '',
			user_category INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL DEFAULT '',
			summary_at TEXT NOT NULL DEFAULT '',
			etag TEXT NOT NULL DEFAULT '',
			last_modified TEXT NOT NULL DEFAULT '',
			indexed_at TEXT NOT NULL,
			PRIMARY KEY (domain, path)
		);
	`)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}
	// Migrations for existing databases.
	_, _ = idx.db.Exec(`ALTER TABLE content_meta ADD COLUMN user_category INTEGER NOT NULL DEFAULT 0`)
	_, _ = idx.db.Exec(`ALTER TABLE content_meta ADD COLUMN summary TEXT NOT NULL DEFAULT ''`)
	_, _ = idx.db.Exec(`ALTER TABLE content_meta ADD COLUMN summary_at TEXT NOT NULL DEFAULT ''`)
	return nil
}

// IndexFile adds or updates a file in the search index.
// Category is derived automatically from path, content type, and body.
func (idx *Index) IndexFile(domain, urlPath, contentType, body string, category ...string) error {
	cat := CatOther
	if len(category) > 0 && category[0] != "" {
		cat = category[0]
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Remove old entry from FTS table
	_, err = tx.Exec(
		"DELETE FROM content WHERE domain = ? AND path = ?",
		domain, urlPath,
	)
	if err != nil {
		return fmt.Errorf("deleting old fts entry: %w", err)
	}

	// Append existing summary to body for FTS searchability
	ftsBody := body
	var existingSummary string
	_ = tx.QueryRow("SELECT summary FROM content_meta WHERE domain = ? AND path = ?", domain, urlPath).Scan(&existingSummary)
	if existingSummary != "" {
		ftsBody = body + "\n\n" + existingSummary
	}

	// Insert new entry
	_, err = tx.Exec(
		"INSERT INTO content (domain, path, content_type, body) VALUES (?, ?, ?, ?)",
		domain, urlPath, contentType, ftsBody,
	)
	if err != nil {
		return fmt.Errorf("inserting fts entry: %w", err)
	}

	// Upsert metadata. If the user has overridden the category (user_category=1),
	// preserve their choice instead of overwriting with the auto-derived one.
	_, err = tx.Exec(`
		INSERT INTO content_meta (domain, path, content_type, size, category, indexed_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(domain, path) DO UPDATE SET
			content_type = excluded.content_type,
			size = excluded.size,
			category = CASE WHEN content_meta.user_category = 1 THEN content_meta.category ELSE excluded.category END,
			indexed_at = excluded.indexed_at
	`, domain, urlPath, contentType, len(body), cat)
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
	Category    string  `json:"category"`
	Snippet     string  `json:"snippet"`
	Summary     string  `json:"summary,omitempty"`
	Rank        float64 `json:"rank"`
}

// SearchOpts controls search behavior.
type SearchOpts struct {
	Site        string // filter to a specific domain
	ContentType string // filter to a content type
	Category    string // filter to a category (e.g. "api-reference")
	Path        string // filter to paths containing this substring
	Limit       int
	Offset      int
}

// Search performs a full-text search across all indexed content.
// Path matches in the query are boosted: if a query term appears in the file
// path, that result ranks higher. This helps agents find e.g. "getting-started"
// docs when searching for "getting started".
func (idx *Index) Search(query string, opts SearchOpts) ([]SearchHit, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "content MATCH ?")
	args = append(args, query)

	if opts.Site != "" {
		conditions = append(conditions, "content.domain = ?")
		args = append(args, opts.Site)
	}
	if opts.ContentType != "" {
		conditions = append(conditions, "content.content_type = ?")
		args = append(args, opts.ContentType)
	}
	if opts.Category != "" {
		conditions = append(conditions, "m.category = ?")
		args = append(args, opts.Category)
	}
	if opts.Path != "" {
		conditions = append(conditions, "content.path LIKE ?")
		args = append(args, "%"+opts.Path+"%")
	}

	where := strings.Join(conditions, " AND ")

	// Build a path-boost expression: for each query word, if it appears in the
	// path (case-insensitive), subtract 5.0 from rank (FTS5 rank is negative,
	// more negative = better). This ensures path matches surface first.
	words := queryWords(query)
	pathBoost := ""
	var boostArgs []any
	if len(words) > 0 {
		var parts []string
		for _, w := range words {
			parts = append(parts, "CASE WHEN LOWER(content.path) LIKE ? THEN -5.0 ELSE 0.0 END")
			boostArgs = append(boostArgs, "%"+strings.ToLower(w)+"%")
		}
		pathBoost = " + " + strings.Join(parts, " + ")
	}

	// Combine all args: boost args first (in SELECT), then filter args, then LIMIT/OFFSET
	allArgs := make([]any, 0, len(boostArgs)+len(args)+2)
	allArgs = append(allArgs, boostArgs...)
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, opts.Limit, opts.Offset)

	q := fmt.Sprintf(`
		SELECT content.domain, content.path, content.content_type,
			COALESCE(m.category, ''),
			snippet(content, 3, '**', '**', '...', 40),
			COALESCE(m.summary, ''),
			content.rank%s AS boosted_rank
		FROM content
		LEFT JOIN content_meta m ON content.domain = m.domain AND content.path = m.path
		WHERE %s
		ORDER BY boosted_rank
		LIMIT ? OFFSET ?
	`, pathBoost, where)

	rows, err := idx.db.Query(q, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.Domain, &h.Path, &h.ContentType, &h.Category, &h.Snippet, &h.Summary, &h.Rank); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// queryWords splits a search query into individual words for path boosting,
// stripping FTS5 operators.
func queryWords(query string) []string {
	var words []string
	for w := range strings.FieldsSeq(query) {
		// Skip FTS5 operators
		upper := strings.ToUpper(w)
		if upper == "AND" || upper == "OR" || upper == "NOT" || upper == "NEAR" {
			continue
		}
		// Strip quotes and prefix operators
		w = strings.Trim(w, `"'*^`)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
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

			cat := categorize(domain, urlPath, ct, string(body))
			return idx.IndexFile(domain, urlPath, ct, string(body), cat)
		})
		if err != nil {
			return fmt.Errorf("indexing %s: %w", domain, err)
		}
	}
	return nil
}

// GetCacheHeaders returns the stored ETag and Last-Modified for a file.
func (idx *Index) GetCacheHeaders(domain, urlPath string) (etag, lastModified string, err error) {
	err = idx.db.QueryRow(
		"SELECT etag, last_modified FROM content_meta WHERE domain = ? AND path = ?",
		domain, urlPath,
	).Scan(&etag, &lastModified)
	if err != nil {
		return "", "", nil // not found is fine — no cache headers
	}
	return etag, lastModified, nil
}

// UpdateCacheHeaders stores ETag and Last-Modified for a file.
func (idx *Index) UpdateCacheHeaders(domain, urlPath, etag, lastModified string) error {
	_, err := idx.db.Exec(
		"UPDATE content_meta SET etag = ?, last_modified = ? WHERE domain = ? AND path = ?",
		etag, lastModified, domain, urlPath,
	)
	return err
}

// GetCategory returns the category for a specific file.
func (idx *Index) GetCategory(domain, urlPath string) (string, error) {
	var cat string
	err := idx.db.QueryRow(
		"SELECT category FROM content_meta WHERE domain = ? AND path = ?",
		domain, urlPath,
	).Scan(&cat)
	if err != nil {
		return "", nil
	}
	return cat, nil
}

// GetContentType returns the content type for a specific file from the index.
func (idx *Index) GetContentType(domain, urlPath string) (string, error) {
	var ct string
	err := idx.db.QueryRow(
		"SELECT content_type FROM content_meta WHERE domain = ? AND path = ?",
		domain, urlPath,
	).Scan(&ct)
	if err != nil {
		return "", nil
	}
	return ct, nil
}

// SetCategory overrides the category for a specific file (agent feedback).
// The override is marked as user-set so re-indexing preserves it.
func (idx *Index) SetCategory(domain, urlPath, category string) error {
	res, err := idx.db.Exec(
		"UPDATE content_meta SET category = ?, user_category = 1 WHERE domain = ? AND path = ?",
		category, domain, urlPath,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no indexed file %s%s", domain, urlPath)
	}
	return nil
}

// GetSummary returns the agent-submitted summary for a file, if any.
func (idx *Index) GetSummary(domain, urlPath string) (summary, summaryAt string, err error) {
	err = idx.db.QueryRow(
		"SELECT summary, summary_at FROM content_meta WHERE domain = ? AND path = ?",
		domain, urlPath,
	).Scan(&summary, &summaryAt)
	if err != nil {
		return "", "", nil
	}
	return summary, summaryAt, nil
}

// SetSummary stores an agent-submitted summary for a file and re-indexes the
// FTS entry so the summary is searchable.
func (idx *Index) SetSummary(domain, urlPath, summary string) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		"UPDATE content_meta SET summary = ?, summary_at = datetime('now') WHERE domain = ? AND path = ?",
		summary, domain, urlPath,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no indexed file %s%s", domain, urlPath)
	}

	// Re-index FTS to include the summary in searchable content.
	// Read the current FTS body (without old summary), append new summary.
	var oldBody string
	err = tx.QueryRow("SELECT body FROM content WHERE domain = ? AND path = ?", domain, urlPath).Scan(&oldBody)
	if err == nil {
		// Strip any previously appended summary (best-effort: trim after double newline at end)
		_, _ = tx.Exec("DELETE FROM content WHERE domain = ? AND path = ?", domain, urlPath)
		var ct string
		_ = tx.QueryRow("SELECT content_type FROM content_meta WHERE domain = ? AND path = ?", domain, urlPath).Scan(&ct)
		ftsBody := oldBody + "\n\n" + summary
		_, _ = tx.Exec("INSERT INTO content (domain, path, content_type, body) VALUES (?, ?, ?, ?)",
			domain, urlPath, ct, ftsBody)
	}

	return tx.Commit()
}

// CategoryCounts returns category distribution for a domain.
func (idx *Index) CategoryCounts(domain string) (map[string]int, error) {
	rows, err := idx.db.Query(
		"SELECT category, COUNT(*) FROM content_meta WHERE domain = ? GROUP BY category",
		domain,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var cat string
		var n int
		if err := rows.Scan(&cat, &n); err != nil {
			return nil, err
		}
		if cat != "" {
			counts[cat] = n
		}
	}
	return counts, rows.Err()
}

// Close closes the database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// DeleteSite removes all index entries for a domain.
func (idx *Index) DeleteSite(domain string) error {
	if _, err := idx.db.Exec("DELETE FROM content WHERE domain = ?", domain); err != nil {
		return err
	}
	_, err := idx.db.Exec("DELETE FROM content_meta WHERE domain = ?", domain)
	return err
}

// ClassifyPath returns a content type string for a URL path.
func ClassifyPath(path string) string {
	return classifyPath(path)
}

func classifyPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, "/llms.txt") || lower == "/llms.txt":
		return "llms-txt"
	case strings.HasSuffix(lower, "/llms-full.txt") || lower == "/llms-full.txt":
		return "llms-full-txt"
	case strings.HasSuffix(lower, "/llms-ctx.txt") || lower == "/llms-ctx.txt":
		return "llms-ctx-txt"
	case strings.HasSuffix(lower, "/llms-ctx-full.txt") || lower == "/llms-ctx-full.txt":
		return "llms-ctx-full-txt"
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
