package discovery

import (
	"bufio"
	"context"
	"strings"
)

// parseCompanions parses an llms.txt file to find referenced companion files.
// It looks for URLs and markdown-style links that point to .md or .html.md files.
func (d *Discoverer) parseCompanions(ctx context.Context, baseURL string, llmsTxt DiscoveredFile) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")

	// We need to fetch the content again (or cache it — for now, re-fetch)
	resp, err := d.Fetcher.Fetch(ctx, llmsTxt.URL)
	if err != nil || resp == nil {
		return nil
	}

	seen := make(map[string]bool)
	var found []DiscoveredFile

	scanner := bufio.NewScanner(strings.NewReader(string(resp.Body)))
	for scanner.Scan() {
		line := scanner.Text()
		urls := extractURLs(line, baseURL)
		for _, u := range urls {
			if seen[u.path] {
				continue
			}
			seen[u.path] = true

			// Probe the URL
			resp, err := d.Fetcher.Fetch(ctx, u.full)
			if err != nil || resp == nil {
				continue
			}
			found = append(found, DiscoveredFile{
				URL:         u.full,
				Path:        u.path,
				ContentType: TypeCompanion,
				Size:        len(resp.Body),
				FoundVia:    "llms.txt reference",
			})
		}
	}

	return found
}

type parsedURL struct {
	full string
	path string
}

// extractURLs finds URLs in a line that look like companion content.
// Handles:
//   - Bare URLs: https://example.com/docs/api.html.md
//   - Markdown links: [API Docs](https://example.com/docs/api.html.md)
//   - Relative paths: /docs/api.html.md or docs/api.html.md
func extractURLs(line, baseURL string) []parsedURL {
	var results []parsedURL

	// Look for markdown-style links: [text](url)
	for {
		idx := strings.Index(line, "](")
		if idx < 0 {
			break
		}
		start := idx + 2
		end := strings.Index(line[start:], ")")
		if end < 0 {
			break
		}
		raw := line[start : start+end]
		line = line[start+end:]

		if u := resolveURL(raw, baseURL); u != nil {
			results = append(results, *u)
		}
	}

	// Also look for bare URLs or paths that end in .md or .txt
	for _, word := range strings.Fields(line) {
		// Strip common punctuation
		word = strings.TrimRight(word, ".,;:)")
		if u := resolveURL(word, baseURL); u != nil {
			// Avoid duplicates from markdown parsing
			dup := false
			for _, r := range results {
				if r.full == u.full {
					dup = true
					break
				}
			}
			if !dup {
				results = append(results, *u)
			}
		}
	}

	return results
}

func resolveURL(raw, baseURL string) *parsedURL {
	// Skip well-known files — they're handled by probeWellKnown
	trimmed := strings.TrimPrefix(raw, baseURL)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "llms.txt" || trimmed == "llms-full.txt" || trimmed == "ai.txt" {
		return nil
	}

	// Must look like content we care about
	if !looksLikeContent(raw) {
		return nil
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		// Absolute URL — extract path
		path := raw
		if strings.HasPrefix(raw, baseURL) {
			path = strings.TrimPrefix(raw, baseURL)
		} else {
			// External URL — skip
			return nil
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return &parsedURL{full: raw, path: path}
	}

	// Relative path
	path := raw
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return &parsedURL{
		full: baseURL + path,
		path: path,
	}
}

func looksLikeContent(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".html.md")
}
