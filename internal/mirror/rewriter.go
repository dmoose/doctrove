package mirror

import (
	"regexp"
	"strings"
)

// RewriteLinks converts absolute URLs pointing to the same domain into relative local paths.
// External links are preserved as-is.
func RewriteLinks(content string, baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	escaped := regexp.QuoteMeta(baseURL)

	re := regexp.MustCompile(escaped + `(/[^\s\)\]"'>]+)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		path := strings.TrimPrefix(match, baseURL)
		return "." + path
	})
}
