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
		{"/runtime/getting_started/first_project", "companion", CatTutorial},
		{"/examples/chat_app_tutorial", "companion", CatTutorial},
		{"/examples/deploy_command_tutorial", "companion", CatTutorial},
		{"/examples/volumes_tutorial", "companion", CatTutorial},

		// Guides
		{"/guides/atlas-guides", "companion", CatGuide},
		{"/docs/learn/architecture.md", "companion", CatGuide},
		{"/deploy", "companion", CatGuide},
		{"/deploy/getting_started", "companion", CatGuide},
		{"/sandbox", "companion", CatGuide},
		{"/sandbox/getting_started", "companion", CatGuide},

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
		{"/contributing", "companion", CatCommunity},
		{"/runtime/contributing", "companion", CatCommunity},
		{"/seps/1686-tasks.md", "companion", CatSpec},

		// Content type shortcuts
		{"/llms.txt", "llms-txt", CatIndex},
		{"/llms-full.txt", "llms-full-txt", CatIndex},
		{"/docs.md", "context7", CatContext7},
		{"/.well-known/agent.json", "well-known", CatOther},
	}

	for _, tt := range tests {
		got := categorize("example.com", tt.path, tt.contentType, "")
		if got != tt.want {
			t.Errorf("Categorize(%q, %q) = %q, want %q", tt.path, tt.contentType, got, tt.want)
		}
	}
}

func TestCategorizeByBody(t *testing.T) {
	// High code block density, terse prose → api-reference
	codeHeavy := "# API\n\n" + strings.Repeat("```go\nfunc Foo() {}\n```\n\nSome text.\n\n", 4)
	got := categorize("example.com", "/docs/foo.md", "companion", codeHeavy)
	if got != CatAPIReference {
		t.Errorf("code-heavy body: got %q, want %q", got, CatAPIReference)
	}

	// Code blocks with lots of prose → tutorial
	tutorialBody := "# Getting Started\n\n" +
		"In this guide, you'll learn how to set up the SDK. Let's get started.\n\n" +
		strings.Repeat(
			"First, follow these steps to configure your project. "+
				"This is a detailed explanation of what each setting does and why you need it. "+
				"Make sure to read through this carefully before proceeding to the next step.\n\n"+
				"```js\nconst x = 1;\n```\n\n", 4)
	got = categorize("example.com", "/docs/foo.md", "companion", tutorialBody)
	if got != CatTutorial {
		t.Errorf("tutorial body (code+prose): got %q, want %q", got, CatTutorial)
	}

	// Link-heavy short lines → marketing
	var lines []string
	for range 30 {
		lines = append(lines, "[Link](http://example.com/page)")
	}
	linkHeavy := strings.Join(lines, "\n")
	got = categorize("example.com", "/products/foo", "companion", linkHeavy)
	if got != CatMarketing {
		t.Errorf("link-heavy body: got %q, want %q", got, CatMarketing)
	}

	// Prose with tutorial signals → tutorial
	tutorialProse := strings.Repeat("In this guide, follow along step by step. You will learn how to build a project. Let's begin with the basics. ", 5)
	got = categorize("example.com", "/docs/overview.md", "companion", tutorialProse)
	if got != CatTutorial {
		t.Errorf("tutorial prose: got %q, want %q", got, CatTutorial)
	}

	// Normal prose → other
	prose := strings.Repeat("This is a normal paragraph with enough text to be meaningful. ", 20)
	got = categorize("example.com", "/docs/overview.md", "companion", prose)
	if got != CatOther {
		t.Errorf("prose body: got %q, want %q", got, CatOther)
	}
}
