package store

import (
	"strings"
	"testing"
)

func TestCategorizePathPatterns(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		want        string
	}{
		// API reference
		{"/api/payments", "companion", CatAPIReference},
		{"/docs/reference/hooks", "companion", CatAPIReference},

		// Tutorials
		{"/docs/tutorials/security/authorization.md", "companion", CatTutorial},
		{"/getting-started/intro.md", "companion", CatTutorial},
		{"/registry/quickstart.md", "companion", CatTutorial},

		// Guides
		{"/guides/atlas-guides", "companion", CatGuide},
		{"/docs/learn/architecture.md", "companion", CatGuide},

		// Spec
		{"/specification/2025-11-25/basic/transports.md", "companion", CatSpec},
		{"/specification/2025-11-25/schema.md", "companion", CatSpec},

		// Changelog
		{"/specification/2025-11-25/changelog.md", "companion", CatChangelog},

		// Marketing
		{"/resources/more/payments", "companion", CatMarketing},
		{"/use-cases/ecommerce", "companion", CatMarketing},
		{"/pricing", "companion", CatMarketing},
		{"/customers", "companion", CatMarketing},
		{"/enterprise", "companion", CatMarketing},
		{"/blog", "companion", CatMarketing},

		// Legal
		{"/privacy", "companion", CatLegal},
		{"/legal/restricted-businesses", "companion", CatLegal},

		// Community
		{"/community/contributing.md", "companion", CatCommunity},
		{"/seps/1686-tasks.md", "companion", CatCommunity},

		// Content type shortcuts
		{"/llms.txt", "llms-txt", CatOther},
		{"/llms-full.txt", "llms-full-txt", CatOther},
		{"/docs.md", "context7", CatContext7},
		{"/.well-known/agent.json", "well-known", CatOther},
	}

	for _, tt := range tests {
		got := Categorize("example.com", tt.path, tt.contentType, "")
		if got != tt.want {
			t.Errorf("Categorize(%q, %q) = %q, want %q", tt.path, tt.contentType, got, tt.want)
		}
	}
}

func TestCategorizeByBody(t *testing.T) {
	// High code block density → api-reference
	codeHeavy := "# API\n\n" + strings.Repeat("```go\nfunc Foo() {}\n```\n\nSome text.\n\n", 4)
	got := Categorize("example.com", "/docs/foo.md", "companion", codeHeavy)
	if got != CatAPIReference {
		t.Errorf("code-heavy body: got %q, want %q", got, CatAPIReference)
	}

	// Link-heavy short lines → marketing
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "[Link](http://example.com/page)")
	}
	linkHeavy := strings.Join(lines, "\n")
	got = Categorize("example.com", "/products/foo", "companion", linkHeavy)
	if got != CatMarketing {
		t.Errorf("link-heavy body: got %q, want %q", got, CatMarketing)
	}

	// Normal prose → other
	prose := strings.Repeat("This is a normal paragraph with enough text to be meaningful. ", 20)
	got = Categorize("example.com", "/docs/overview.md", "companion", prose)
	if got != CatOther {
		t.Errorf("prose body: got %q, want %q", got, CatOther)
	}
}
