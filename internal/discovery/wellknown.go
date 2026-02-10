package discovery

import (
	"context"
	"strings"

	"github.com/dmoose/llmshadow/internal/fetcher"
)

// wellKnownPaths are the standard locations for LLM-targeted content.
var wellKnownPaths = []struct {
	Path      string
	Type      ContentType
	AllowJSON bool // if true, skip HTML check (JSON endpoints)
}{
	{"/llms.txt", TypeLLMSTxt, false},
	{"/llms-full.txt", TypeLLMSFull, false},
	{"/llms-ctx.txt", TypeLLMSCtx, false},
	{"/llms-ctx-full.txt", TypeLLMSCtxFull, false},
	{"/ai.txt", TypeAITxt, false},
	{"/.well-known/tdmrep.json", TypeTDMRep, true},
	{"/.well-known/agent.json", TypeWellKnown, true},
	{"/.well-known/agents.json", TypeWellKnown, true},
}

// probeWellKnown checks all standard well-known locations for LLM content.
func (d *Discoverer) probeWellKnown(ctx context.Context, baseURL string) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")
	var found []DiscoveredFile

	for _, wk := range wellKnownPaths {
		url := baseURL + wk.Path
		// Check robots.txt if enabled
		if d.Robots != nil && !d.Robots.IsAllowed(ctx, url) {
			continue
		}
		resp, err := d.Fetcher.Fetch(ctx, url)
		if err != nil || resp == nil {
			continue
		}
		// If HTML at a content URL, convert to markdown
		body := resp.Body
		foundVia := "well-known"
		if !wk.AllowJSON && fetcher.IsHTML(resp.ContentType, body) {
			md, err := fetcher.ConvertHTML(string(body))
			if err != nil || len(strings.TrimSpace(md)) < 50 {
				continue // conversion failed or produced trivial content
			}
			body = []byte(md)
			foundVia = "well-known (html-to-md)"
		}
		found = append(found, DiscoveredFile{
			URL:         url,
			Path:        wk.Path,
			ContentType: wk.Type,
			Size:        len(body),
			FoundVia:    foundVia,
		})
	}

	return found
}
