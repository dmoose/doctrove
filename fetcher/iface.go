package fetcher

import "context"

// HTTPFetcher is the interface for HTTP content retrieval.
// The default implementation provides per-domain rate limiting and conditional
// requests. Alternative implementations can add browser rendering (Playwright),
// authentication, or custom transport logic.
type HTTPFetcher interface {
	Fetch(ctx context.Context, url string) (*Response, error)
	FetchConditional(ctx context.Context, url, etag, lastModified string) (*Response, error)
}

// Verify that Fetcher satisfies HTTPFetcher at compile time.
var _ HTTPFetcher = (*Fetcher)(nil)
