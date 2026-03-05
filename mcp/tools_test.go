package mcp

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"30m", 30 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"", 0, true},
		{"abc", 0, true},
		{"xd", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDuration(tt.input)
		if tt.err {
			if err == nil {
				t.Errorf("parseDuration(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDuration(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDiffStat(t *testing.T) {
	diff := `diff --git a/sites/example.com/docs/api.md b/sites/example.com/docs/api.md
--- a/sites/example.com/docs/api.md
+++ b/sites/example.com/docs/api.md
@@ -1,3 +1,5 @@
 # API
+New line 1
+New line 2
 existing line
-removed line
diff --git a/sites/example.com/llms.txt b/sites/example.com/llms.txt
new file mode 100644
--- /dev/null
+++ b/sites/example.com/llms.txt
@@ -0,0 +1,3 @@
+# LLMs
+Line 1
+Line 2
`
	entries := diffStat(diff)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First file: 2 added, 1 removed
	if entries[0].File != "example.com/docs/api.md" {
		t.Errorf("file[0] = %q", entries[0].File)
	}
	if entries[0].Added != 2 {
		t.Errorf("file[0] added = %d, want 2", entries[0].Added)
	}
	if entries[0].Removed != 1 {
		t.Errorf("file[0] removed = %d, want 1", entries[0].Removed)
	}

	// Second file: 3 added, 0 removed
	if entries[1].Added != 3 {
		t.Errorf("file[1] added = %d, want 3", entries[1].Added)
	}
	if entries[1].Removed != 0 {
		t.Errorf("file[1] removed = %d, want 0", entries[1].Removed)
	}
}

func TestDiffStatEmpty(t *testing.T) {
	entries := diffStat("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty diff, got %d", len(entries))
	}
}

func TestFilterDiffToContent(t *testing.T) {
	diff := `diff --git a/sites/example.com/llms.txt b/sites/example.com/llms.txt
+++ b/sites/example.com/llms.txt
+content
diff --git a/doctrove.yaml b/doctrove.yaml
+++ b/doctrove.yaml
+config change
diff --git a/sites/example.com/_meta/discovered.json b/sites/example.com/_meta/discovered.json
+++ b/sites/example.com/_meta/discovered.json
+meta change
diff --git a/.doctrove.lock b/.doctrove.lock
+lock
`
	filtered := filterDiffToContent(diff)

	if !strings.Contains(filtered, "llms.txt") {
		t.Error("content file should be kept")
	}
	if strings.Contains(filtered, "doctrove.yaml") {
		t.Error("config file should be filtered")
	}
	if strings.Contains(filtered, "_meta/") {
		t.Error("metadata should be filtered")
	}
	if strings.Contains(filtered, ".doctrove.lock") {
		t.Error("lock file should be filtered")
	}
}

func TestFilterDiffToContentEmpty(t *testing.T) {
	filtered := filterDiffToContent("")
	if filtered != "" {
		t.Errorf("expected empty output for empty input, got %q", filtered)
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name     string
		site     string
		path     string
		err      error
		wantSafe string // substring that MUST appear
		wantGone string // substring that MUST NOT appear (filesystem path)
	}{
		{
			name:     "file not found strips filesystem path",
			site:     "example.com",
			path:     "/docs.md",
			err:      fmt.Errorf("reading example.com/docs.md: open /home/user/.config/doctrove/sites/example.com/docs.md: no such file or directory"),
			wantSafe: "file not found: example.com/docs.md",
			wantGone: "/home/user",
		},
		{
			name:     "section not found passes through",
			site:     "example.com",
			path:     "/docs.md",
			err:      fmt.Errorf("reading example.com/docs.md: section not found: \"API\""),
			wantSafe: "reading example.com/docs.md",
			wantGone: "",
		},
		{
			name:     "bare no such file",
			site:     "example.com",
			path:     "/missing",
			err:      fmt.Errorf("open /var/data/sites/example.com/missing: no such file or directory"),
			wantSafe: "file not found: example.com/missing",
			wantGone: "/var/data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeError(tt.site, tt.path, tt.err)
			if !strings.Contains(got, tt.wantSafe) {
				t.Errorf("sanitizeError() = %q, want substring %q", got, tt.wantSafe)
			}
			if tt.wantGone != "" && strings.Contains(got, tt.wantGone) {
				t.Errorf("sanitizeError() = %q, should NOT contain %q", got, tt.wantGone)
			}
		})
	}
}

func TestInvalidCategoryMsg(t *testing.T) {
	msg := invalidCategoryMsg("bogus")
	if !strings.Contains(msg, `"bogus"`) {
		t.Errorf("expected message to contain the invalid category, got %q", msg)
	}
	if !strings.Contains(msg, "api-reference") {
		t.Errorf("expected message to list valid categories, got %q", msg)
	}
}
