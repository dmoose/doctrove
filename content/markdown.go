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

	// Collect all headings with their indices and levels.
	type heading struct {
		text  string
		lower string
		line  int
		level int
	}
	var headings []heading
	inCodeBlock := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock || !strings.HasPrefix(trimmed, "#") {
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
		h := strings.TrimSpace(trimmed[level:])
		if h == "" {
			continue
		}
		headings = append(headings, heading{text: h, lower: strings.ToLower(h), line: i, level: level})
	}

	// Find best match: prefer exact match, then exact-case-insensitive,
	// then substring on deeper (narrower) headings first.
	bestIdx := -1
	bestScore := -1 // higher is better
	for i, h := range headings {
		score := 0
		switch {
		case h.lower == sectionLower:
			score = 1000 + h.level // exact match, prefer deeper
		case strings.Contains(h.lower, sectionLower):
			score = h.level // substring match, prefer deeper (narrower)
		default:
			continue
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return "", fmt.Errorf("%w: %q", ErrSectionNotFound, sectionName)
	}

	startLine := headings[bestIdx].line
	startLevel := headings[bestIdx].level

	// Find end: next heading at same or higher level
	for j := bestIdx + 1; j < len(headings); j++ {
		if headings[j].level <= startLevel {
			return strings.Join(lines[startLine:headings[j].line], "\n"), nil
		}
	}
	return strings.Join(lines[startLine:], "\n"), nil
}
