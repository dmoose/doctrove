package discovery

import (
	"context"
	"strings"
	"time"

	"github.com/dmoose/doctrove/internal/fetcher"
	"github.com/dmoose/doctrove/internal/robots"
)

// ContentType classifies discovered content.
type ContentType string

const (
	TypeLLMSTxt     ContentType = "llms-txt"
	TypeLLMSFull    ContentType = "llms-full-txt"
	TypeLLMSCtx     ContentType = "llms-ctx-txt"
	TypeLLMSCtxFull ContentType = "llms-ctx-full-txt"
	TypeAITxt       ContentType = "ai-txt"
	TypeCompanion   ContentType = "companion"
	TypeTDMRep      ContentType = "tdmrep"
	TypeWellKnown   ContentType = "well-known"
)

// DiscoveredFile represents a single piece of discovered LLM content.
type DiscoveredFile struct {
	URL         string      `json:"url"`
	Path        string      `json:"path"`         // URL path (e.g., "/llms.txt")
	ContentType ContentType `json:"content_type"`
	Size        int         `json:"size"`
	FoundVia    string      `json:"found_via"`    // How it was discovered
	Body        []byte      `json:"-"`            // If non-nil, content is already fetched (skip mirror fetch)
}

// Result holds everything discovered for a site.
type Result struct {
	Domain       string           `json:"domain"`
	BaseURL      string           `json:"base_url"`
	Files        []DiscoveredFile `json:"files"`
	DiscoveredAt time.Time        `json:"discovered_at"`
}

// Provider discovers LLM-friendly content from a source.
// Implementations handle different content origins — websites, doc aggregators,
// package registries, etc.
type Provider interface {
	// Name returns a short identifier for this provider (e.g., "site", "context7").
	Name() string
	// CanHandle returns true if this provider can discover content for the given input.
	CanHandle(input string) bool
	// Discover probes the input and returns discovered content.
	// Files may include Body (content already fetched) or leave it nil (mirror will fetch).
	Discover(ctx context.Context, input string) (*Result, error)
}

// Discoverer routes discovery requests to registered providers.
type Discoverer struct {
	providers []Provider
}

// New creates a Discoverer with a SiteProvider as the default.
func New(f *fetcher.Fetcher, robotsChecker *robots.Checker, maxProbes int) *Discoverer {
	return &Discoverer{
		providers: []Provider{
			NewSiteProvider(f, robotsChecker, maxProbes),
		},
	}
}

// NewWithProviders creates a Discoverer with the given providers.
func NewWithProviders(providers ...Provider) *Discoverer {
	return &Discoverer{providers: providers}
}

// RegisterProvider adds a provider to the discoverer.
func (d *Discoverer) RegisterProvider(p Provider) {
	d.providers = append(d.providers, p)
}

// Providers returns the registered providers.
func (d *Discoverer) Providers() []Provider {
	return d.providers
}

// Discover routes to the first provider that can handle the input.
func (d *Discoverer) Discover(ctx context.Context, input string) (*Result, error) {
	for _, p := range d.providers {
		if p.CanHandle(input) {
			return p.Discover(ctx, input)
		}
	}
	return &Result{DiscoveredAt: time.Now()}, nil
}

// SiteProvider probes websites for LLM-targeted content at well-known paths.
type SiteProvider struct {
	Fetcher   *fetcher.Fetcher
	Robots    *robots.Checker
	MaxProbes int
}

// NewSiteProvider creates a SiteProvider.
func NewSiteProvider(f *fetcher.Fetcher, robotsChecker *robots.Checker, maxProbes int) *SiteProvider {
	if maxProbes <= 0 {
		maxProbes = 100
	}
	return &SiteProvider{Fetcher: f, Robots: robotsChecker, MaxProbes: maxProbes}
}

func (p *SiteProvider) Name() string { return "site" }

func (p *SiteProvider) CanHandle(input string) bool {
	return len(input) > 8 && (input[:7] == "http://" || input[:8] == "https://")
}

// Discover probes a site for all LLM content.
func (p *SiteProvider) Discover(ctx context.Context, baseURL string) (*Result, error) {
	domain := domainFromURL(baseURL)
	result := &Result{
		Domain:       domain,
		BaseURL:      baseURL,
		DiscoveredAt: time.Now(),
	}

	// Phase 1: well-known locations
	wellKnown := p.probeWellKnown(ctx, baseURL)
	result.Files = append(result.Files, wellKnown...)

	// Build seen set from well-known results
	seen := make(map[string]bool)
	for _, f := range wellKnown {
		seen[f.Path] = true
	}

	// Phase 2: parse llms.txt for companion file references.
	// Skip files that were converted from HTML — those are not real llms.txt
	// index files, just web pages served at the /llms.txt path (e.g. Next.js apps).
	for _, f := range wellKnown {
		if f.ContentType == TypeLLMSTxt && !strings.Contains(f.FoundVia, "html-to-md") {
			companions := p.parseCompanions(ctx, baseURL, f)
			for _, c := range companions {
				seen[c.Path] = true
			}
			result.Files = append(result.Files, companions...)
		}
	}

	// Phase 3: check sitemap for additional LLM content
	sitemapFiles := p.probeSitemap(ctx, baseURL, seen)
	result.Files = append(result.Files, sitemapFiles...)

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
