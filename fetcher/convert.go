package fetcher

import (
	"fmt"
	"regexp"
	"strings"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// MinConvertedLines is the minimum number of non-blank lines for converted
// content to be considered meaningful (vs. a nav-heavy marketing page).
const MinConvertedLines = 10

// ConvertHTML converts HTML content to markdown. It first cleans the HTML by
// extracting the main content area (stripping nav, footer, sidebar), then
// converts to markdown with whitespace normalization.
//
// contentSelectors and removeSelectors are optional — pass nil for defaults.
// Used when a URL that should serve text/markdown returns HTML instead.
func ConvertHTML(html string, selectors ...[]string) (string, error) {
	// Extract content selectors if provided
	var contentSel, removeSel []string
	if len(selectors) > 0 {
		contentSel = selectors[0]
	}
	if len(selectors) > 1 {
		removeSel = selectors[1]
	}

	// Phase 1: Clean HTML — extract main content, remove chrome
	cleaned, err := CleanHTML(html, contentSel, removeSel)
	if err != nil {
		// Fall back to full page if cleaning fails
		cleaned = html
	}

	// Phase 2: Convert cleaned HTML to markdown
	md, err := htmltomd.ConvertString(cleaned)
	if err != nil {
		return "", err
	}

	// Phase 3: Whitespace normalization
	md = cleanWhitespace(md)

	// Phase 4: Quality check
	if looksLikeNavPage(md) {
		return "", fmt.Errorf("content appears to be a navigation page, not documentation")
	}

	return md, nil
}

// reMultiNewlines matches 3+ consecutive newlines.
var reMultiNewlines = regexp.MustCompile(`\n{3,}`)

// reSpaceBeforePunct matches whitespace before punctuation.
var reSpaceBeforePunct = regexp.MustCompile(`\s+([.,;:!?])`)

// cleanWhitespace normalizes markdown output for consistent diffs and readability.
func cleanWhitespace(content string) string {
	// Trim trailing whitespace per line
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	content = strings.Join(lines, "\n")

	// Normalize multiple blank lines to max 2
	content = reMultiNewlines.ReplaceAllString(content, "\n\n")

	// Ensure file ends with single newline
	content = strings.TrimRight(content, "\n") + "\n"

	// Remove spaces before punctuation
	content = reSpaceBeforePunct.ReplaceAllString(content, "$1")

	return content
}

// looksLikeNavPage returns true if the markdown is mostly short lines
// (navigation links, buttons) rather than prose content.
func looksLikeNavPage(md string) bool {
	lines := strings.Split(md, "\n")
	var contentLines, shortLines int
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" {
			continue
		}
		contentLines++
		if len(trimmed) < 40 {
			shortLines++
		}
	}
	if contentLines < MinConvertedLines {
		return true
	}
	// If >70% of lines are very short, it's likely navigation
	return contentLines > 0 && float64(shortLines)/float64(contentLines) > 0.7
}
