package engine

import (
	"bufio"
	"context"
	"strings"
)

// CatalogEntry is a compact summary of a tracked site's LLM content,
// derived from its llms.txt structure.
type CatalogEntry struct {
	Domain      string   `json:"domain"`
	URL         string   `json:"url"`
	Title       string   `json:"title,omitempty"`       // H1 from llms.txt
	Description string   `json:"description,omitempty"` // Blockquote from llms.txt
	Topics      []string `json:"topics,omitempty"`      // H2 sections from llms.txt
	FileCount   int      `json:"file_count"`
}

// Catalog returns a compact summary of all tracked sites, extracting
// title, description, and topic structure from each site's llms.txt.
func (e *Engine) Catalog(ctx context.Context) ([]CatalogEntry, error) {
	var entries []CatalogEntry

	for domain, siteCfg := range e.Config.Sites {
		entry := CatalogEntry{
			Domain: domain,
			URL:    siteCfg.URL,
		}

		count, _ := e.Store.SiteFileCount(domain)
		entry.FileCount = count

		// Parse llms.txt for structure
		body, err := e.Store.ReadContent(domain, "/llms.txt")
		if err == nil {
			entry.Title, entry.Description, entry.Topics = parseLLMSTxt(string(body))
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// parseLLMSTxt extracts the title (H1), description (blockquote), and
// topics (H2 headings) from an llms.txt file following the spec.
func parseLLMSTxt(content string) (title, description string, topics []string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var inBlockquote bool

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// H1 — title
		if title == "" && strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			title = strings.TrimPrefix(trimmed, "# ")
			continue
		}

		// Blockquote — description
		if after, ok := strings.CutPrefix(trimmed, "> "); ok {
			desc := after
			if inBlockquote {
				description += " " + desc
			} else {
				description = desc
				inBlockquote = true
			}
			continue
		}
		if inBlockquote && trimmed == "" {
			inBlockquote = false
		}

		// H2 — topics
		if after, ok := strings.CutPrefix(trimmed, "## "); ok {
			topic := after
			topics = append(topics, topic)
		}
	}

	return
}
