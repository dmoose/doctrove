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

	summary, _, _ := e.Index.GetSummary(domain, path)

	return &OutlineResult{
		Domain:    domain,
		Path:      path,
		TotalSize: len(text),
		Summary:   summary,
		Sections:  result.Sections,
		Truncated: result.Truncated,
	}, nil
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
