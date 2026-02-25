package fetcher

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ContentSelectors are tried in order to find the main content element.
// The first match wins. These cover the major doc platforms.
var DefaultContentSelectors = []string{
	"article",
	".markdown",
	".md-content",
	".theme-doc-markdown",
	".document",
	"[role='main']",
	"main",
	".content",
	"#content",
	".main-content",
}

// DefaultRemoveSelectors strips elements that add noise to converted markdown.
var DefaultRemoveSelectors = []string{
	"nav",
	".navbar",
	".sidebar",
	".md-sidebar",
	".md-header",
	".md-footer",
	".md-source-file",
	".md-content__button",
	".md-edit",
	".headerlink",
	".docSidebarContainer",
	".pagination-nav",
	".theme-doc-footer",
	".sphinxsidebarwrapper",
	".advertisement",
	"footer",
	"script",
	"style",
}

// CleanHTML extracts the main content from an HTML page using CSS selectors,
// removes navigation/chrome elements, and returns the cleaned HTML ready for
// markdown conversion. This dramatically improves conversion quality compared
// to converting the full page.
//
// If contentSelectors is nil, DefaultContentSelectors is used.
// If removeSelectors is nil, DefaultRemoveSelectors is used.
func CleanHTML(html string, contentSelectors, removeSelectors []string) (string, error) {
	if len(contentSelectors) == 0 {
		contentSelectors = DefaultContentSelectors
	}
	if len(removeSelectors) == 0 {
		removeSelectors = DefaultRemoveSelectors
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	// Find main content via selector priority
	var content *goquery.Selection
	for _, sel := range contentSelectors {
		content = doc.Find(sel).First()
		if content.Length() > 0 {
			break
		}
	}

	// Fallback to body
	if content == nil || content.Length() == 0 {
		content = doc.Find("body").First()
		if content.Length() == 0 {
			return html, nil // give up, return original
		}
	}

	// Remove unwanted elements
	for _, sel := range removeSelectors {
		content.Find(sel).Remove()
	}

	cleaned, err := content.Html()
	if err != nil {
		return "", fmt.Errorf("extracting HTML: %w", err)
	}
	return cleaned, nil
}
