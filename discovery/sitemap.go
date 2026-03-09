package discovery

import (
	"context"
	"encoding/xml"
	"strings"

	"github.com/dmoose/doctrove/fetcher"
)

type sitemapIndex struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

// probeSitemap checks sitemap.xml for URLs that look like LLM content
// (paths ending in .md, .txt, or containing /llms/).
func (p *SiteProvider) probeSitemap(ctx context.Context, baseURL string, seen map[string]bool) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")
	sitemapURL := baseURL + "/sitemap.xml"

	resp, err := p.Fetcher.Fetch(ctx, sitemapURL)
	if err != nil || resp == nil {
		return nil
	}

	var sitemap sitemapIndex
	if err := xml.Unmarshal(resp.Body, &sitemap); err != nil {
		return nil
	}

	var found []DiscoveredFile
	probes := 0

	for _, u := range sitemap.URLs {
		if probes >= p.MaxProbes {
			break
		}
		loc := u.Loc
		if !isLLMContent(loc, baseURL) {
			continue
		}

		path := strings.TrimPrefix(loc, baseURL)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		probes++

		// Check robots if enabled
		if p.Robots != nil && !p.Robots.IsAllowed(ctx, loc) {
			continue
		}

		r, err := p.Fetcher.Fetch(ctx, loc)
		if err != nil || r == nil {
			continue
		}

		body := r.Body
		foundVia := "sitemap"
		if fetcher.IsHTML(r.ContentType, body) {
			md, err := fetcher.ConvertHTML(string(body))
			if err != nil || len(strings.TrimSpace(md)) < 50 {
				continue
			}
			body = []byte(md)
			foundVia = "sitemap (html-to-md)"
		}

		found = append(found, DiscoveredFile{
			URL:         loc,
			Path:        path,
			ContentType: TypeCompanion,
			Size:        len(body),
			FoundVia:    foundVia,
		})
	}

	return found
}

// isLLMContent returns true if a sitemap URL looks like LLM-targeted content.
func isLLMContent(loc, baseURL string) bool {
	// Must be same domain
	if !strings.HasPrefix(loc, baseURL) {
		return false
	}

	lower := strings.ToLower(loc)
	// Paths containing /llms/ are likely LLM content
	if strings.Contains(lower, "/llms/") {
		return true
	}
	// Content file extensions
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".html.md")
}
