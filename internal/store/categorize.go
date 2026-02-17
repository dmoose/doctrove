package store

import "strings"

// Content categories classify pages by their purpose, enabling agents to filter
// search results by task (e.g. coding agents want api-reference, not marketing).
const (
	CatAPIReference = "api-reference"
	CatTutorial     = "tutorial"
	CatGuide        = "guide"
	CatSpec         = "spec"
	CatChangelog    = "changelog"
	CatMarketing    = "marketing"
	CatLegal        = "legal"
	CatCommunity    = "community"
	CatContext7     = "context7"
	CatOther        = "other"
)

// Categorize assigns a semantic category to a page based on its path, content
// type, and body content. Path patterns are checked first (fast), then body
// heuristics as a fallback.
func Categorize(domain, path, contentType, body string) string {
	lower := strings.ToLower(path)

	// Content type shortcuts
	if contentType == "context7" {
		return CatContext7
	}
	if contentType == "llms-txt" || contentType == "llms-full-txt" ||
		contentType == "llms-ctx-txt" || contentType == "llms-ctx-full-txt" ||
		contentType == "ai-txt" || contentType == "tdmrep" || contentType == "well-known" {
		return CatOther
	}

	// Path-based classification (order matters — first match wins).
	// More specific patterns go first to avoid false matches
	// (e.g. changelog before spec, so /specification/.../changelog.md → changelog).
	pathRules := []struct {
		patterns []string
		category string
	}{
		{[]string{"/changelog", "/release-notes", "/releases"}, CatChangelog},
		{[]string{"/api/", "/reference/", "/api-reference/"}, CatAPIReference},
		{[]string{"/tutorial/", "/tutorials/", "/getting-started/", "/quickstart"}, CatTutorial},
		{[]string{"/guide/", "/guides/", "/learn/", "/how-to/"}, CatGuide},
		{[]string{"/spec/", "/specification/", "/schema"}, CatSpec},
		{[]string{"/resources/more/", "/use-cases/", "/pricing", "/customers", "/industries/",
			"/enterprise", "/startups", "/sessions", "/blog", "/newsroom"}, CatMarketing},
		{[]string{"/privacy", "/legal/", "/terms", "/restricted-businesses", "/licenses"}, CatLegal},
		{[]string{"/community/", "/seps/", "/contributing", "/governance"}, CatCommunity},
	}

	for _, rule := range pathRules {
		for _, pattern := range rule.patterns {
			if strings.Contains(lower, pattern) {
				return rule.category
			}
		}
	}

	// Body heuristics for companion pages with generic paths
	if contentType == "companion" && len(body) > 100 {
		return categorizeByBody(body)
	}

	return CatOther
}

// categorizeByBody uses content signals when path patterns don't match.
func categorizeByBody(body string) string {
	// Each fenced code block has an opening and closing ```, so count pairs.
	codeMarkers := strings.Count(body, "```")
	codeBlocks := codeMarkers / 2
	lines := strings.Count(body, "\n") + 1

	// High code-block density suggests API reference or tutorial
	if codeBlocks >= 3 {
		return CatAPIReference
	}

	// Very short lines on average with lots of links suggests marketing/nav
	if lines > 20 {
		linkCount := strings.Count(body, "](")
		avgLineLen := len(body) / lines
		if avgLineLen < 60 && linkCount > lines/3 {
			return CatMarketing
		}
	}

	return CatOther
}
