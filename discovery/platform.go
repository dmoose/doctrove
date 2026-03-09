package discovery

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Platform identifies a documentation site's framework/theme.
type Platform struct {
	Theme            string   `json:"theme"`             // mkdocs-material, docusaurus, sphinx, generic
	ContentSelectors []string `json:"content_selectors"` // CSS selectors for main content
	RemoveSelectors  []string `json:"remove_selectors"`  // CSS selectors for chrome to strip
}

// DetectPlatform inspects HTML to identify the documentation platform and
// returns appropriate CSS selectors for content extraction. This improves
// HTML→markdown conversion quality by targeting the actual content area.
func DetectPlatform(html string) Platform {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return genericPlatform()
	}

	theme := detectTheme(doc)
	content, remove := selectorsForTheme(theme)
	return Platform{
		Theme:            theme,
		ContentSelectors: content,
		RemoveSelectors:  remove,
	}
}

func detectTheme(doc *goquery.Document) string {
	// MkDocs Material
	if doc.Find(".md-content").Length() > 0 || doc.Find(".md-sidebar").Length() > 0 {
		return "mkdocs-material"
	}
	// Docusaurus
	if doc.Find(".theme-doc-markdown").Length() > 0 ||
		doc.Find(".docSidebarContainer").Length() > 0 ||
		doc.Find("meta[name='generator'][content*='Docusaurus']").Length() > 0 {
		return "docusaurus"
	}
	// Sphinx / ReadTheDocs
	if doc.Find(".document").Length() > 0 || doc.Find("[role='main']").Length() > 0 {
		return "sphinx"
	}
	// GitBook
	if doc.Find(".gitbook-root").Length() > 0 || doc.Find("[data-testid='page.contentEditor']").Length() > 0 {
		return "gitbook"
	}
	return "generic"
}

func selectorsForTheme(theme string) (content []string, remove []string) {
	switch theme {
	case "mkdocs-material":
		return []string{"article", ".md-content", "main"},
			[]string{"nav", ".md-header", ".md-footer", ".md-sidebar", ".md-source-file", ".md-content__button", ".headerlink", ".md-edit", "footer"}
	case "docusaurus":
		return []string{"article", ".markdown", "main"},
			[]string{"nav", ".navbar", ".pagination-nav", ".docSidebarContainer", ".theme-doc-footer", "footer"}
	case "sphinx":
		return []string{".document", "article", "[role='main']", "main"},
			[]string{".sphinxsidebarwrapper", ".sidebar", ".headerlink", "nav", "footer"}
	case "gitbook":
		return []string{"main", "[data-testid='page.contentEditor']", "article"},
			[]string{"nav", "aside", "header", "footer"}
	default:
		return genericPlatform().ContentSelectors, genericPlatform().RemoveSelectors
	}
}

func genericPlatform() Platform {
	return Platform{
		Theme:            "generic",
		ContentSelectors: []string{"main", "article", ".content", "#content", ".main-content"},
		RemoveSelectors:  []string{"nav", ".sidebar", "footer", ".advertisement", "script", "style"},
	}
}
