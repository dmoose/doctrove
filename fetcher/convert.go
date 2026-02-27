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

// reMDXComponent matches self-closing and block MDX/JSX component tags
// like <Card>, <Note>, <Tab>, <Frame>, <Warning>, <Info>, <Step>, etc.
// These are framework artifacts that add noise to mirrored content.
var reMDXComponent = regexp.MustCompile(`</?(?:Card|Note|Tab|Tabs|Frame|Warning|Info|Step|Steps|Tip|Accordion|AccordionGroup|CardGroup|ResponseField|Expandable|ParamField|CodeGroup)\b[^>]*>`)

// reMDXExport matches MDX export statements (export const/function/default).
var reMDXExport = regexp.MustCompile(`(?m)^export\s+(?:const|function|default|let|var)\b[^\n]*(?:\n(?:[ \t].*|\{.*\}.*|.*\);?\s*))*`)

// reMDXDiv matches div tags with id attributes commonly used for MDX features.
var reMDXDiv = regexp.MustCompile(`<div\s+id="[^"]*"\s*/?>`)

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

// CleanMDX strips MDX/JSX component tags and export statements from markdown
// content. These are framework artifacts (Mintlify, Nextra, etc.) that add
// noise without contributing documentation value.
func CleanMDX(content string) string {
	// Strip MDX component tags
	content = reMDXComponent.ReplaceAllString(content, "")
	// Strip export statements
	content = reMDXExport.ReplaceAllString(content, "")
	// Strip MDX div markers
	content = reMDXDiv.ReplaceAllString(content, "")
	// Strip Documentation Index banners (added by some sites to every page)
	content = stripDocIndexBanner(content)
	// Clean up resulting whitespace
	content = reMultiNewlines.ReplaceAllString(content, "\n\n")
	return strings.TrimLeft(content, "\n")
}

// stripDocIndexBanner removes common "fetch the documentation index" blockquote
// banners that some sites inject into every companion page.
func stripDocIndexBanner(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "> ## Documentation Index") ||
			strings.HasPrefix(trimmed, "> Fetch the complete documentation index") ||
			strings.HasPrefix(trimmed, "> Use this file to discover all available pages") {
			skip = true
			continue
		}
		if skip && (trimmed == "" || strings.HasPrefix(trimmed, ">")) {
			if trimmed == "" {
				skip = false
			}
			continue
		}
		skip = false
		result = append(result, line)
	}
	return strings.Join(result, "\n")
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
