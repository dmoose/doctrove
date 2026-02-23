package store

// Indexer is the interface for full-text search backends.
// The default implementation uses SQLite FTS5. Alternative implementations
// could provide semantic/vector search or different storage engines.
type Indexer interface {
	IndexFile(domain, path, contentType, body string, category ...string) error
	Search(query string, opts SearchOpts) ([]SearchHit, error)
	DeleteSite(domain string) error
	Rebuild(store *Store) error
	GetCacheHeaders(domain, path string) (etag, lastModified string, err error)
	UpdateCacheHeaders(domain, path, etag, lastModified string) error
	GetCategory(domain, path string) (string, error)
	SetCategory(domain, path, category string) error
	GetSummary(domain, path string) (summary, summaryAt string, err error)
	SetSummary(domain, path, summary string) error
	CategoryCounts(domain string) (map[string]int, error)
	Close() error
}

// Verify that Index implements Indexer at compile time.
var _ Indexer = (*Index)(nil)
