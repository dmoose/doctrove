package discovery

import "context"

// ContentDiscoverer probes URLs for LLM-targeted content.
// The default implementation tries well-known paths, companion files,
// sitemaps, and optional Context7 API. Alternative implementations could
// add platform detection, deeper crawling, or custom discovery strategies.
type ContentDiscoverer interface {
	Discover(ctx context.Context, input string) (*Result, error)
	RegisterProvider(p Provider)
	Providers() []Provider
}

// Verify that Discoverer satisfies ContentDiscoverer at compile time.
var _ ContentDiscoverer = (*Discoverer)(nil)
