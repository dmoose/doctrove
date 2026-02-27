package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/dmoose/doctrove/content"
)

// OutlineResult is the table of contents for a file.
type OutlineResult struct {
	Domain    string            `json:"domain"`
	Path      string            `json:"path"`
	TotalSize int               `json:"total_size"`
	Summary   string            `json:"summary,omitempty"`
	Hint      string            `json:"hint,omitempty"`
	Sections  []content.Section `json:"sections"`
	Truncated bool              `json:"truncated,omitempty"`
}

// Outline parses a mirrored file and returns its heading structure.
// maxDepth limits heading levels (0 = no limit, default 3 recommended).
// maxSections caps entries returned (0 = no limit, default 100 recommended).
func (e *Engine) Outline(ctx context.Context, domain, path string, maxDepth, maxSections int) (*OutlineResult, error) {
	body, err := e.Store.ReadContent(domain, path)
	if err != nil {
		return nil, fmt.Errorf("reading %s%s: %w", domain, path, err)
	}

	text := string(body)
	proc := e.processorFor(path, "")
	result := proc.Outline(text, maxDepth, maxSections)

	// Get unfiltered count to generate accurate hints when maxDepth filters sections
	var unfilteredCount int
	if maxDepth > 0 || maxSections > 0 {
		unfiltered := proc.Outline(text, 0, 0)
		unfilteredCount = len(unfiltered.Sections)
	} else {
		unfilteredCount = len(result.Sections)
	}

	summary, _, _ := e.Index.GetSummary(domain, path)

	out := &OutlineResult{
		Domain:    domain,
		Path:      path,
		TotalSize: len(text),
		Summary:   summary,
		Sections:  result.Sections,
		Truncated: result.Truncated,
	}

	// Hint for degenerate files with no sub-headings
	if unfilteredCount <= 1 && len(text) > 5000 {
		out.Hint = "This file has no sub-headings for section-based reading. Use trove_read with max_lines to preview, or trove_search to find specific content within it."
	} else if len(result.Sections) < unfilteredCount && maxDepth > 0 {
		out.Hint = fmt.Sprintf("Showing %d of %d sections (max_depth=%d). Increase max_depth to see deeper headings.", len(result.Sections), unfilteredCount, maxDepth)
	}

	return out, nil
}

// ReadSection reads a specific section of a file by heading match, or a line range.
// If section is non-empty, delegates to the content processor.
// If section is empty and maxLines > 0, returns the first maxLines lines.
func (e *Engine) ReadSection(ctx context.Context, domain, path, section string, maxLines int) (string, error) {
	body, err := e.Store.ReadContent(domain, path)
	if err != nil {
		return "", fmt.Errorf("reading %s%s: %w", domain, path, err)
	}

	text := string(body)
	if section == "" && maxLines <= 0 {
		return text, nil
	}

	if section != "" {
		proc := e.processorFor(path, "")
		return proc.ReadSection(text, section)
	}

	// Line-limited read.
	lines := strings.Split(text, "\n")
	if maxLines > len(lines) {
		maxLines = len(lines)
	}
	return strings.Join(lines[:maxLines], "\n"), nil
}

// processorFor returns the first content processor that can handle the path,
// falling back to the last registered processor (markdown by default).
func (e *Engine) processorFor(path, contentType string) content.Processor {
	for _, p := range e.Processors {
		if p.CanProcess(path, contentType) {
			return p
		}
	}
	return &content.MarkdownProcessor{}
}
