package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultUserAgent    = "llmshadow/0.1"
	defaultRatePerHost  = 2 // requests per second per host
	defaultBurstPerHost = 5
	defaultTimeout      = 30 * time.Second
)

// Fetcher handles all HTTP requests with per-domain rate limiting.
type Fetcher struct {
	client   *http.Client
	limiters map[string]*rate.Limiter
	mu       sync.Mutex
}

// Response holds the result of a fetch.
type Response struct {
	StatusCode int
	Body       []byte
	ETag       string
	URL        string
}

// New creates a Fetcher with default settings.
func New() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
		limiters: make(map[string]*rate.Limiter),
	}
}

// Fetch retrieves the content at the given URL.
// Returns nil Response (not an error) for 404s, so callers can treat missing content as expected.
func (f *Fetcher) Fetch(ctx context.Context, url string) (*Response, error) {
	if err := f.limiterFor(url).Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		ETag:       resp.Header.Get("ETag"),
		URL:        url,
	}, nil
}

func (f *Fetcher) limiterFor(rawURL string) *rate.Limiter {
	// Extract host from URL for per-domain limiting
	host := extractHost(rawURL)

	f.mu.Lock()
	defer f.mu.Unlock()

	if lim, ok := f.limiters[host]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(defaultRatePerHost), defaultBurstPerHost)
	f.limiters[host] = lim
	return lim
}

func extractHost(rawURL string) string {
	// Simple extraction — just find the host portion
	// Avoid importing net/url for this hot path
	start := 0
	if i := indexOf(rawURL, "://"); i >= 0 {
		start = i + 3
	}
	end := len(rawURL)
	if i := indexOfFrom(rawURL, '/', start); i >= 0 {
		end = i
	}
	return rawURL[start:end]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func indexOfFrom(s string, c byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
