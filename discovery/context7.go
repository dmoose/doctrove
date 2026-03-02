package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	context7BaseURL = "https://context7.com/api/v2"
	// TypeContext7 classifies content fetched from Context7.
	TypeContext7 ContentType = "context7"
)

// Context7Provider discovers LLM-optimized documentation via the Context7 API.
// It resolves library names to IDs, then fetches curated documentation snippets.
type Context7Provider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	retryBackoff time.Duration // initial backoff between retries (default 2s)
}

// NewContext7Provider creates a Context7Provider.
// apiKey should start with "ctx7sk". If empty, the provider will still
// be registered but all requests will fail with an auth error from the API.
func NewContext7Provider(apiKey string) *Context7Provider {
	return &Context7Provider{
		apiKey:  apiKey,
		baseURL: context7BaseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Context7Provider) Name() string { return "context7" }

// CanHandle returns true for bare library names (not URLs).
// Inputs like "react", "stripe-node", "nextjs" match.
// URLs (http:// or https://) do not match.
func (p *Context7Provider) CanHandle(input string) bool {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return false
	}
	// Must look like a library name: non-empty, no spaces (allow / for scoped packages)
	input = strings.TrimSpace(input)
	return len(input) > 0 && !strings.Contains(input, " ")
}

// Discover resolves the library name, fetches docs, and returns them with Body populated.
func (p *Context7Provider) Discover(ctx context.Context, input string) (*Result, error) {
	input = strings.TrimSpace(input)

	// Phase 1: resolve library ID
	libs, err := p.searchLibrary(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("context7 search: %w", err)
	}
	if len(libs) == 0 {
		return &Result{
			Domain:       "context7.com",
			BaseURL:      p.baseURL,
			DiscoveredAt: time.Now(),
		}, nil
	}

	lib := libs[0] // best match

	// Phase 2: fetch documentation as text
	docs, err := p.getContext(ctx, lib.ID, input)
	if err != nil {
		return nil, fmt.Errorf("context7 docs for %s: %w", lib.ID, err)
	}

	// Use a unique domain per library so multiple C7 libraries can be tracked.
	// e.g., "context7.com~facebook_react" for library ID "/facebook/react"
	safeName := strings.ReplaceAll(strings.TrimPrefix(lib.ID, "/"), "/", "_")
	domain := "context7.com~" + safeName

	result := &Result{
		Domain:       domain,
		BaseURL:      p.baseURL,
		DiscoveredAt: time.Now(),
	}

	if len(docs) > 0 {
		result.Files = append(result.Files, DiscoveredFile{
			URL:         fmt.Sprintf("%s/context?libraryId=%s", p.baseURL, url.QueryEscape(lib.ID)),
			Path:        "/docs.md",
			ContentType: TypeContext7,
			Size:        len(docs),
			FoundVia:    fmt.Sprintf("context7 (%s, %d snippets)", lib.DisplayName(), lib.TotalSnippets),
			Body:        docs,
		})
	}

	return result, nil
}

// c7Library represents a library from the search endpoint.
type c7Library struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"` // legacy field, fallback for Title
	Description   string  `json:"description"`
	TotalSnippets int     `json:"totalSnippets"`
	TotalTokens   int     `json:"totalTokens"`
	TrustScore    float64 `json:"trustScore"`
}

// DisplayName returns Title if set, otherwise Name.
func (l c7Library) DisplayName() string {
	if l.Title != "" {
		return l.Title
	}
	return l.Name
}

func (p *Context7Provider) searchLibrary(ctx context.Context, name string) ([]c7Library, error) {
	u := fmt.Sprintf("%s/libs/search?libraryName=%s&query=%s",
		p.baseURL,
		url.QueryEscape(name),
		url.QueryEscape(name),
	)

	body, err := p.doGet(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []c7Library `json:"results"`
		Error   string      `json:"error"`
		Message string      `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing search response: %w (body: %s)", err, truncate(string(body), 200))
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("context7: %s", resp.Message)
	}
	return resp.Results, nil
}

func (p *Context7Provider) getContext(ctx context.Context, libraryID, query string) ([]byte, error) {
	u := fmt.Sprintf("%s/context?libraryId=%s&query=%s&type=txt",
		p.baseURL,
		url.QueryEscape(libraryID),
		url.QueryEscape(query),
	)

	return p.doGet(ctx, u)
}

func (p *Context7Provider) doGet(ctx context.Context, rawURL string) ([]byte, error) {
	const maxRetries = 3
	backoff := p.retryBackoff
	if backoff == 0 {
		backoff = 2 * time.Second
	}

	for attempt := range maxRetries {
		body, status, err := p.doGetOnce(ctx, rawURL)
		if err != nil {
			return nil, err
		}

		switch {
		case status == http.StatusOK:
			return body, nil
		case status == http.StatusAccepted || status == http.StatusTooManyRequests || status >= 500:
			// Retryable — wait and try again
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("HTTP %d after %d attempts: %s", status, maxRetries, truncate(string(body), 200))
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		default:
			return nil, fmt.Errorf("HTTP %d: %s", status, truncate(string(body), 200))
		}
	}
	return nil, fmt.Errorf("unreachable")
}

func (p *Context7Provider) doGetOnce(ctx context.Context, rawURL string) (body []byte, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("reading response: %w", err)
	}
	return body, resp.StatusCode, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
