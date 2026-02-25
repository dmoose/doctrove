package fetcher

import (
	"regexp"
	"strings"
)

// jsIndicators are patterns that suggest a page requires JavaScript rendering.
// Two or more matches → JS-heavy.
var jsIndicators = []string{
	// Apple DocC
	"<script>var baseurl =",
	"chunk-vendors",

	// SPA mount points
	`<div id="app"></div>`,
	`<div id="root"></div>`,
	`<div id="__next"`,
	"<div ng-app",

	// JS-required hints
	"<noscript>",
	"this page requires javascript",
	"please enable javascript",
	"you need to enable javascript",

	// Build tool artifacts
	"webpack",
	"/_next/static/",
	"/js/app.",
	"/js/chunk-",
	`defer="defer"`,
	`defer=""`,

	// Module-era / Vite / Rollup
	`type="module"`,
	`rel="modulepreload"`,
	"/assets/",
	"vite",
	"rollup",
	"import.meta",
}

// reTagStrip is compiled once for stripping HTML tags in body-text estimation.
var reTagStrip = regexp.MustCompile(`<[^>]+>`)

// IsJSHeavy detects whether an HTML page is a JavaScript SPA shell that would
// need browser rendering to extract meaningful content.
//
// Uses a two-tier strategy:
//  1. Short-circuit for obvious SPA shells (mount div + module hint + tiny body text).
//  2. Score indicators — returns true if >=2 match (conservative).
func IsJSHeavy(html string) bool {
	lo := strings.ToLower(html)

	// Short-circuit: modern SPA shell (mount div + module hint + almost no text)
	if hasMountDiv(lo) && hasModuleHint(lo) && hasTinyBodyText(lo) {
		return true
	}

	// Scored indicators — need >=2 to flag
	hits := 0
	for _, p := range jsIndicators {
		if strings.Contains(lo, p) {
			hits++
			if hits >= 2 {
				return true
			}
		}
	}
	return false
}

func hasMountDiv(lo string) bool {
	return strings.Contains(lo, `<div id="root">`) ||
		strings.Contains(lo, `<div id="app">`) ||
		strings.Contains(lo, `<div id="__next"`)
}

func hasModuleHint(lo string) bool {
	return strings.Contains(lo, `type="module"`) ||
		strings.Contains(lo, `rel="modulepreload"`) ||
		strings.Contains(lo, "/assets/") ||
		strings.Contains(lo, "vite") ||
		strings.Contains(lo, "rollup") ||
		strings.Contains(lo, "import.meta")
}

// hasTinyBodyText estimates visible text by stripping tags. If under ~1KB,
// the page is likely a JS shell with no server-rendered content.
func hasTinyBodyText(lo string) bool {
	text := reTagStrip.ReplaceAllString(lo, " ")
	text = strings.TrimSpace(strings.ReplaceAll(text, "&nbsp;", " "))
	return len(text) < 1024
}
