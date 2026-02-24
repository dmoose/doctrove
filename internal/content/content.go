// Package content defines interfaces for parsing and processing document content.
package content

import "fmt"

// Section represents a structural division of a document (e.g. a heading).
type Section struct {
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	Line    int    `json:"line"`       // 1-based line number
	Size    int    `json:"size_chars"` // characters in this section
}

// OutlineResult holds the parsed structure of a document.
type OutlineResult struct {
	Sections  []Section `json:"sections"`
	Truncated bool      `json:"truncated,omitempty"`
}

// Processor parses document content into structural components.
// Implementations exist for different content formats (markdown, etc.).
type Processor interface {
	// Name returns the processor identifier (e.g. "markdown").
	Name() string

	// CanProcess returns true if this processor handles the given path/content type.
	CanProcess(path, contentType string) bool

	// Outline extracts the heading structure from content.
	// maxDepth limits heading levels (0 = no limit).
	// maxSections caps the number of returned sections (0 = no limit).
	Outline(content string, maxDepth, maxSections int) OutlineResult

	// ReadSection extracts the content under a named heading.
	// Uses case-insensitive substring matching on heading text.
	ReadSection(content, sectionName string) (string, error)
}

// ErrSectionNotFound is returned when a section heading cannot be matched.
var ErrSectionNotFound = fmt.Errorf("section not found")
