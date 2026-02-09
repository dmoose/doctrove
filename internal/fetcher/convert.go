package fetcher

import (
	"fmt"
	"strings"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// MinConvertedLines is the minimum number of non-blank lines for converted
// content to be considered meaningful (vs. a nav-heavy marketing page).
const MinConvertedLines = 10

// ConvertHTML converts HTML content to markdown. Used when a URL that should
// serve text/markdown returns HTML instead (common with Next.js, SPA frameworks).
func ConvertHTML(html string) (string, error) {
	md, err := htmltomd.ConvertString(html)
	if err != nil {
		return "", err
	}
	// Clean up excessive whitespace from conversion
	lines := strings.Split(md, "\n")
	var cleaned []string
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount <= 2 {
				cleaned = append(cleaned, "")
			}
			continue
		}
		blankCount = 0
		cleaned = append(cleaned, line)
	}
	result := strings.TrimSpace(strings.Join(cleaned, "\n")) + "\n"

	// Quality check: reject if it looks like a navigation/marketing page
	// rather than actual documentation content
	if looksLikeNavPage(result) {
		return "", fmt.Errorf("content appears to be a navigation page, not documentation")
	}

	return result, nil
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
