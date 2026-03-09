package discovery

import (
	"context"
	"strings"
	"time"

	"github.com/dmoose/doctrove/fetcher"
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
	Path        string      `json:"path"` // URL path (e.g., "/llms.txt")
	ContentType ContentType `json:"content_type"`
	Size        int         `json:"size"`
	FoundVia    string      `json:"found_via"` // How it was discovered
	Body        []byte      `json:"-"`         // If non-nil, content is already fetched (skip mirror fetch)
}

// Result holds everything discovered for a site.
type Result struct {
	Domain       string           `json:"domain"`
	BaseURL      string           `json:"base_url"`
	Files        []DiscoveredFile `json:"files"`
	ProbedPaths  []string         `json:"probed_paths,omitempty"` // paths that were checked (shown when no files found)
	Platform     *Platform        `json:"platform,omitempty"`     // detected doc platform/theme
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
func New(f fetcher.HTTPFetcher, robotsChecker *robots.Checker, maxProbes int) *Discoverer {
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
	Fetcher   fetcher.HTTPFetcher
	Robots    *robots.Checker
	MaxProbes int
}

// NewSiteProvider creates a SiteProvider.
func NewSiteProvider(f fetcher.HTTPFetcher, robotsChecker *robots.Checker, maxProbes int) *SiteProvider {
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

	// Phase 4: if no llms.txt or companions found, probe common doc seed paths.
	// Also detect the doc platform from the index page for better HTML cleaning.
	if len(result.Files) == 0 {
		seedFiles := p.probeSeedPaths(ctx, baseURL, seen)
		result.Files = append(result.Files, seedFiles...)
	}

	// Platform detection: fetch index page and identify the doc framework.
	// This runs on first discovery to inform HTML cleaning selectors.
	if platform := p.detectPlatform(ctx, baseURL); platform != nil {
		result.Platform = platform
	}

	// When no files found, report what we probed so agents know we actually checked
	if len(result.Files) == 0 {
		base := strings.TrimRight(baseURL, "/")
		probed := make([]string, 0, len(wellKnownPaths)+len(seedPaths)+1)
		for _, wk := range wellKnownPaths {
			probed = append(probed, base+wk.Path)
		}
		probed = append(probed, base+"/sitemap.xml")
		for _, sp := range seedPaths {
			probed = append(probed, base+sp)
		}
		result.ProbedPaths = probed
	}

	return result, nil
}

// seedPaths are common documentation entry points tried when no llms.txt is found.
var seedPaths = []string{
	"/docs",
	"/docs/",
	"/getting-started",
	"/introduction",
	"/overview",
	"/guide",
	"/quickstart",
}

// probeSeedPaths probes common documentation paths when a site has no llms.txt.
func (p *SiteProvider) probeSeedPaths(ctx context.Context, baseURL string, seen map[string]bool) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")
	var found []DiscoveredFile

	for _, seed := range seedPaths {
		if seen[seed] {
			continue
		}
		url := baseURL + seed
		if p.Robots != nil && !p.Robots.IsAllowed(ctx, url) {
			continue
		}
		resp, err := p.Fetcher.Fetch(ctx, url)
		if err != nil || resp == nil {
			continue
		}
		if fetcher.IsHTML(resp.ContentType, resp.Body) {
			if fetcher.IsJSHeavy(string(resp.Body)) {
				continue // skip JS-only shells
			}
			md, convErr := fetcher.ConvertHTML(string(resp.Body))
			if convErr != nil || len(strings.TrimSpace(md)) < 50 {
				continue
			}
			found = append(found, DiscoveredFile{
				URL:         url,
				Path:        seed,
				ContentType: TypeCompanion,
				Size:        len(md),
				FoundVia:    "seed-probe (html-to-md)",
				Body:        []byte(md),
			})
			seen[seed] = true
		}
	}
	return found
}

// detectPlatform fetches the site index and identifies the documentation platform.
func (p *SiteProvider) detectPlatform(ctx context.Context, baseURL string) *Platform {
	resp, err := p.Fetcher.Fetch(ctx, strings.TrimRight(baseURL, "/")+"/")
	if err != nil || resp == nil {
		return nil
	}
	if !fetcher.IsHTML(resp.ContentType, resp.Body) {
		return nil
	}
	platform := DetectPlatform(string(resp.Body))
	return &platform
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
