package content

import (
	"errors"
	"testing"
)

var proc = &MarkdownProcessor{}

const testDoc = `# Title

Intro paragraph.

## Getting Started

Some setup instructions.

### Prerequisites

You need Go 1.21+.

### Installation

Run go install.

## API Reference

### Create

POST /api/things

### Delete

DELETE /api/things/:id

## Changelog

- v1.0: initial release
`

func TestOutlineBasic(t *testing.T) {
	result := proc.Outline(testDoc, 0, 0)
	if len(result.Sections) != 8 {
		t.Fatalf("expected 8 sections, got %d", len(result.Sections))
	}
	if result.Sections[0].Heading != "Title" || result.Sections[0].Level != 1 {
		t.Errorf("first section = %+v", result.Sections[0])
	}
	if result.Truncated {
		t.Error("should not be truncated")
	}
}

func TestOutlineMaxDepth(t *testing.T) {
	result := proc.Outline(testDoc, 2, 0)
	for _, s := range result.Sections {
		if s.Level > 2 {
			t.Errorf("section %q has level %d, want <= 2", s.Heading, s.Level)
		}
	}
	// Should have: Title(1), Getting Started(2), API Reference(2), Changelog(2) = 4
	if len(result.Sections) != 4 {
		t.Fatalf("expected 4 sections at depth<=2, got %d", len(result.Sections))
	}
}

func TestOutlineMaxSections(t *testing.T) {
	result := proc.Outline(testDoc, 0, 3)
	if len(result.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(result.Sections))
	}
	if !result.Truncated {
		t.Error("should be truncated")
	}
}

func TestOutlineEmpty(t *testing.T) {
	result := proc.Outline("no headings here\njust text", 0, 0)
	if len(result.Sections) != 0 {
		t.Fatalf("expected 0 sections, got %d", len(result.Sections))
	}
}

func TestOutlineSizeCalculation(t *testing.T) {
	result := proc.Outline(testDoc, 0, 0)
	totalSize := 0
	for _, s := range result.Sections {
		if s.Size <= 0 {
			t.Errorf("section %q has size %d", s.Heading, s.Size)
		}
		totalSize += s.Size
	}
	// Total of all sections should approximate the full doc size.
	// (First section starts at line 1, so should cover everything.)
	if totalSize < len(testDoc)-10 {
		t.Errorf("total section size %d much less than doc size %d", totalSize, len(testDoc))
	}
}

func TestReadSectionByName(t *testing.T) {
	text, err := proc.ReadSection(testDoc, "API Reference")
	if err != nil {
		t.Fatalf("ReadSection: %v", err)
	}
	if !containsLine(text, "## API Reference") {
		t.Error("result should start with the heading")
	}
	if !containsLine(text, "POST /api/things") {
		t.Error("result should include Create subsection")
	}
	// Should NOT include content from the next H2 (Changelog).
	if containsLine(text, "v1.0: initial release") {
		t.Error("result should not include Changelog content")
	}
}

func TestReadSectionCaseInsensitive(t *testing.T) {
	text, err := proc.ReadSection(testDoc, "getting started")
	if err != nil {
		t.Fatalf("ReadSection: %v", err)
	}
	if !containsLine(text, "## Getting Started") {
		t.Error("should find section case-insensitively")
	}
}

func TestReadSectionSubstring(t *testing.T) {
	text, err := proc.ReadSection(testDoc, "Prerequis")
	if err != nil {
		t.Fatalf("ReadSection: %v", err)
	}
	if !containsLine(text, "### Prerequisites") {
		t.Error("should match substring")
	}
}

func TestReadSectionNotFound(t *testing.T) {
	_, err := proc.ReadSection(testDoc, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing section")
	}
	if !errors.Is(err, ErrSectionNotFound) {
		t.Errorf("expected ErrSectionNotFound, got %v", err)
	}
}

func TestReadSectionLastSection(t *testing.T) {
	text, err := proc.ReadSection(testDoc, "Changelog")
	if err != nil {
		t.Fatalf("ReadSection: %v", err)
	}
	if !containsLine(text, "v1.0: initial release") {
		t.Error("last section should extend to end of doc")
	}
}

func containsLine(text, substr string) bool {
	for _, line := range splitLines(text) {
		if line == substr || contains(line, substr) {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	lines := []string{}
	for _, l := range split(s) {
		lines = append(lines, l)
	}
	return lines
}

func split(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func contains(s, sub string) bool {
	return len(sub) <= len(s) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
