package discovery

import (
	"context"
	"strings"
)

// wellKnownPaths are the standard locations for LLM-targeted content.
var wellKnownPaths = []struct {
	Path string
	Type ContentType
}{
	{"/llms.txt", TypeLLMSTxt},
	{"/llms-full.txt", TypeLLMSFull},
	{"/ai.txt", TypeAITxt},
}

// probeWellKnown checks all standard well-known locations for LLM content.
func (d *Discoverer) probeWellKnown(ctx context.Context, baseURL string) []DiscoveredFile {
	baseURL = strings.TrimRight(baseURL, "/")
	var found []DiscoveredFile

	for _, wk := range wellKnownPaths {
		url := baseURL + wk.Path
		resp, err := d.Fetcher.Fetch(ctx, url)
		if err != nil || resp == nil {
			continue
		}
		found = append(found, DiscoveredFile{
			URL:         url,
			Path:        wk.Path,
			ContentType: wk.Type,
			Size:        len(resp.Body),
			FoundVia:    "well-known",
		})
	}

	return found
}
