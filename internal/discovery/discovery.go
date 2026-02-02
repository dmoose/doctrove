package discovery

import (
	"context"
	"time"

	"github.com/dmoose/llmshadow/internal/fetcher"
)

// ContentType classifies discovered content.
type ContentType string

const (
	TypeLLMSTxt    ContentType = "llms-txt"
	TypeLLMSFull   ContentType = "llms-full-txt"
	TypeAITxt      ContentType = "ai-txt"
	TypeCompanion  ContentType = "companion"
)

// DiscoveredFile represents a single piece of discovered LLM content.
type DiscoveredFile struct {
	URL         string      `json:"url"`
	Path        string      `json:"path"`         // URL path (e.g., "/llms.txt")
	ContentType ContentType `json:"content_type"`
	Size        int         `json:"size"`
	FoundVia    string      `json:"found_via"`    // How it was discovered
}

// Result holds everything discovered for a site.
type Result struct {
	Domain       string           `json:"domain"`
	BaseURL      string           `json:"base_url"`
	Files        []DiscoveredFile `json:"files"`
	DiscoveredAt time.Time        `json:"discovered_at"`
}

// Discoverer probes a site for LLM-targeted content.
type Discoverer struct {
	Fetcher *fetcher.Fetcher
}

// New creates a Discoverer.
func New(f *fetcher.Fetcher) *Discoverer {
	return &Discoverer{Fetcher: f}
}

// Discover probes a site for all LLM content.
func (d *Discoverer) Discover(ctx context.Context, baseURL string) (*Result, error) {
	domain := domainFromURL(baseURL)
	result := &Result{
		Domain:       domain,
		BaseURL:      baseURL,
		DiscoveredAt: time.Now(),
	}

	// Phase 1: well-known locations
	wellKnown := d.probeWellKnown(ctx, baseURL)
	result.Files = append(result.Files, wellKnown...)

	// Phase 2: parse llms.txt for companion file references
	for _, f := range wellKnown {
		if f.ContentType == TypeLLMSTxt {
			companions := d.parseCompanions(ctx, baseURL, f)
			result.Files = append(result.Files, companions...)
		}
	}

	return result, nil
}

func domainFromURL(rawURL string) string {
	start := 0
	if i := findStr(rawURL, "://"); i >= 0 {
		start = i + 3
	}
	end := len(rawURL)
	if i := findByteFrom(rawURL, '/', start); i >= 0 {
		end = i
	}
	return rawURL[start:end]
}

func findStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func findByteFrom(s string, c byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
