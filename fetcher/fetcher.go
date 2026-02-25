package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Fetcher handles all HTTP requests with per-domain rate limiting.
type Fetcher struct {
	client      *http.Client
	limiters    map[string]*rate.Limiter
	mu          sync.Mutex
	userAgent   string
	ratePerHost int
	burstPerHost int
}

// Response holds the result of a fetch.
type Response struct {
	StatusCode   int
	Body         []byte
	ETag         string
	LastModified string
	URL          string
	ContentType  string
}

// Options configures the Fetcher.
type Options struct {
	UserAgent   string
	RatePerHost int
	BurstPerHost int
	Timeout     time.Duration
}

// New creates a Fetcher with the given options.
func New(opts Options) *Fetcher {
	if opts.UserAgent == "" {
		opts.UserAgent = "doctrove/0.1"
	}
	if opts.RatePerHost <= 0 {
		opts.RatePerHost = 2
	}
	if opts.BurstPerHost <= 0 {
		opts.BurstPerHost = 5
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	return &Fetcher{
		client: &http.Client{
			Timeout: opts.Timeout,
		},
		limiters:    make(map[string]*rate.Limiter),
		userAgent:   opts.UserAgent,
		ratePerHost: opts.RatePerHost,
		burstPerHost: opts.BurstPerHost,
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
	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

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
		StatusCode:   resp.StatusCode,
		Body:         body,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		URL:          url,
		ContentType:  resp.Header.Get("Content-Type"),
	}, nil
}

// FetchConditional retrieves content only if it has changed, using ETag and
// Last-Modified headers for cache validation. Returns nil, nil on 304 Not
// Modified (same pattern as 404).
func (f *Fetcher) FetchConditional(ctx context.Context, url, etag, lastModified string) (*Response, error) {
	if etag == "" && lastModified == "" {
		return f.Fetch(ctx, url) // no cache headers, do a normal fetch
	}

	if err := f.limiterFor(url).Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil // content unchanged
	}
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
		StatusCode:   resp.StatusCode,
		Body:         body,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		URL:          url,
		ContentType:  resp.Header.Get("Content-Type"),
	}, nil
}

func (f *Fetcher) limiterFor(rawURL string) *rate.Limiter {
	host := extractHost(rawURL)

	f.mu.Lock()
	defer f.mu.Unlock()

	if lim, ok := f.limiters[host]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(f.ratePerHost), f.burstPerHost)
	f.limiters[host] = lim
	return lim
}

func extractHost(rawURL string) string {
	start := 0
	if i := strings.Index(rawURL, "://"); i >= 0 {
		start = i + 3
	}
	end := len(rawURL)
	if i := strings.IndexByte(rawURL[start:], '/'); i >= 0 {
		end = start + i
	}
	return rawURL[start:end]
}

// IsHTML returns true if the response looks like an HTML page rather than
// text/markdown content. Checks both Content-Type header and body sniffing.
func IsHTML(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	lower := bytes.ToLower(trimmed[:min(len(trimmed), 256)])
	return bytes.HasPrefix(lower, []byte("<!doctype")) ||
		bytes.HasPrefix(lower, []byte("<html")) ||
		bytes.HasPrefix(lower, []byte("<head"))
}
