package discovery

import (
	"bytes"
	"context"
	"strings"

	"github.com/dmoose/doctrove/fetcher"
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
func (p *SiteProvider) probeWellKnown(ctx context.Context, baseURL string) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")
	var found []DiscoveredFile

	for _, wk := range wellKnownPaths {
		url := baseURL + wk.Path
		// Check robots.txt if enabled
		if p.Robots != nil && !p.Robots.IsAllowed(ctx, url) {
			continue
		}
		resp, err := p.Fetcher.Fetch(ctx, url)
		if err != nil || resp == nil {
			continue
		}
		body := resp.Body
		foundVia := "well-known"

		// For JSON endpoints, validate the response is actually JSON
		if wk.AllowJSON {
			ct := strings.ToLower(resp.ContentType)
			if !strings.Contains(ct, "json") && !looksLikeJSON(body) {
				continue // likely an HTML error page, not real JSON
			}
		}

		// If HTML at a content URL, convert to markdown
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

// looksLikeJSON returns true if the body starts with { or [ after trimming whitespace.
func looksLikeJSON(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}
