package discovery

import (
	"bufio"
	"context"
	"strings"

	"github.com/dmoose/llmshadow/internal/fetcher"
)

// maxCompanionProbes default; overridden by Discoverer.MaxProbes

// parseCompanions parses an llms.txt file to find referenced companion files.
// Markdown links are followed permissively (same-domain links in llms.txt are
// intentional), while bare URLs require a recognized file extension. All fetched
// content is validated to reject HTML pages.
func (d *Discoverer) parseCompanions(ctx context.Context, baseURL string, llmsTxt DiscoveredFile) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")

	resp, err := d.Fetcher.Fetch(ctx, llmsTxt.URL)
	if err != nil || resp == nil {
		return nil
	}

	seen := make(map[string]bool)
	var found []DiscoveredFile
	probes := 0

	scanner := bufio.NewScanner(strings.NewReader(string(resp.Body)))
	for scanner.Scan() {
		line := scanner.Text()
		urls := extractURLs(line, baseURL)
		for _, u := range urls {
			if probes >= d.MaxProbes {
				return found
			}
			if seen[u.path] {
				continue
			}
			seen[u.path] = true
			probes++

			// Check robots.txt if enabled
			if d.Robots != nil && !d.Robots.IsAllowed(ctx, u.full) {
				continue
			}
			resp, err := d.Fetcher.Fetch(ctx, u.full)
			if err != nil || resp == nil {
				continue
			}
			// If HTML, try converting to markdown
			body := resp.Body
			foundVia := "llms.txt reference"
			if fetcher.IsHTML(resp.ContentType, body) {
				md, err := fetcher.ConvertHTML(string(body))
				if err != nil || len(strings.TrimSpace(md)) < 50 {
					continue
				}
				body = []byte(md)
				foundVia = "llms.txt reference (html-to-md)"
			}
			found = append(found, DiscoveredFile{
				URL:         u.full,
				Path:        u.path,
				ContentType: TypeCompanion,
				Size:        len(body),
				FoundVia:    foundVia,
			})
		}
	}

	return found
}

type parsedURL struct {
	full string
	path string
}

// extractURLs finds URLs in a line of llms.txt content.
// Markdown links [text](url) are followed permissively for same-domain targets.
// Bare URLs require a recognized content extension (.md, .txt, .json).
func extractURLs(line, baseURL string) []parsedURL {
	var results []parsedURL

	remaining := line
	// Extract markdown-style links: [text](url)
	for {
		idx := strings.Index(remaining, "](")
		if idx < 0 {
			break
		}
		start := idx + 2
		end := strings.Index(remaining[start:], ")")
		if end < 0 {
			break
		}
		raw := remaining[start : start+end]
		remaining = remaining[start+end:]

		// Markdown links in llms.txt are curated — follow permissively
		if u := resolveURLPermissive(raw, baseURL); u != nil {
			results = append(results, *u)
		}
	}

	// Bare URLs/paths require recognized extensions
	for _, word := range strings.Fields(remaining) {
		word = strings.TrimRight(word, ".,;:)")
		if u := resolveURLStrict(word, baseURL); u != nil {
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

// resolveURLPermissive resolves a URL from a markdown link. Any same-domain or
// relative URL is accepted (HTML will be rejected at fetch time via Content-Type check).
func resolveURLPermissive(raw, baseURL string) *parsedURL {
	raw = normalizeRaw(raw)
	if raw == "" {
		return nil
	}

	if isWellKnownPath(raw, baseURL) {
		return nil
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if !strings.HasPrefix(raw, baseURL) {
			return nil // external — skip
		}
		path := strings.TrimPrefix(raw, baseURL)
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
	return &parsedURL{full: baseURL + path, path: path}
}

// resolveURLStrict resolves a bare URL, requiring a recognized content extension.
func resolveURLStrict(raw, baseURL string) *parsedURL {
	raw = normalizeRaw(raw)
	if raw == "" {
		return nil
	}

	if !looksLikeContent(raw) {
		return nil
	}

	return resolveURLPermissive(raw, baseURL)
}

// normalizeRaw cleans up a raw URL string.
func normalizeRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	// Handle ./ relative prefix
	if strings.HasPrefix(raw, "./") {
		raw = raw[1:] // "./foo" -> "/foo"
	}
	return raw
}

// isWellKnownPath returns true if the path matches a file handled by probeWellKnown.
func isWellKnownPath(raw, baseURL string) bool {
	trimmed := strings.TrimPrefix(raw, baseURL)
	trimmed = strings.TrimPrefix(trimmed, "/")
	switch trimmed {
	case "llms.txt", "llms-full.txt", "llms-ctx.txt", "llms-ctx-full.txt", "ai.txt":
		return true
	}
	return false
}

// looksLikeContent checks if a URL path has a recognized content extension.
func looksLikeContent(s string) bool {
	lower := strings.ToLower(s)
	// Strip query string and fragment before checking
	if i := strings.IndexAny(lower, "?#"); i >= 0 {
		lower = lower[:i]
	}
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".json")
}
