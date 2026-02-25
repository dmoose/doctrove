package content

import (
	"fmt"
	"strings"
)

// MarkdownProcessor parses markdown documents by ATX heading structure.
type MarkdownProcessor struct{}

var _ Processor = (*MarkdownProcessor)(nil)

func (m *MarkdownProcessor) Name() string { return "markdown" }

func (m *MarkdownProcessor) CanProcess(path, contentType string) bool {
	// Markdown is the default/fallback processor — handles everything.
	return true
}

func (m *MarkdownProcessor) Outline(text string, maxDepth, maxSections int) OutlineResult {
	lines := strings.Split(text, "\n")
	var sections []Section
	truncated := false
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for _, c := range trimmed {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		if level < 1 || level > 6 {
			continue
		}
		if maxDepth > 0 && level > maxDepth {
			continue
		}
		heading := strings.TrimSpace(trimmed[level:])
		if heading == "" {
			continue
		}
		if maxSections > 0 && len(sections) >= maxSections {
			truncated = true
			break
		}
		sections = append(sections, Section{
			Heading: heading,
			Level:   level,
			Line:    i + 1, // 1-based
		})
	}

	// Compute section sizes: chars from this heading to the next section in the result.
	for i := range sections {
		startLine := sections[i].Line - 1
		var endLine int
		if i+1 < len(sections) {
			endLine = sections[i+1].Line - 1
		} else {
			endLine = len(lines)
		}
		size := 0
		for l := startLine; l < endLine && l < len(lines); l++ {
			size += len(lines[l]) + 1
		}
		sections[i].Size = size
	}

	return OutlineResult{Sections: sections, Truncated: truncated}
}

func (m *MarkdownProcessor) ReadSection(text, sectionName string) (string, error) {
	lines := strings.Split(text, "\n")
	sectionLower := strings.ToLower(sectionName)
	startIdx := -1
	startLevel := 0
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for _, c := range trimmed {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		heading := strings.TrimSpace(trimmed[level:])
		if startIdx == -1 {
			if strings.Contains(strings.ToLower(heading), sectionLower) {
				startIdx = i
				startLevel = level
			}
		} else if level <= startLevel {
			return strings.Join(lines[startIdx:i], "\n"), nil
		}
	}

	if startIdx >= 0 {
		return strings.Join(lines[startIdx:], "\n"), nil
	}
	return "", fmt.Errorf("%w: %q", ErrSectionNotFound, sectionName)
}
