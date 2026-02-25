package content

import "context"

// Summarizer generates summaries for document content.
// Implementations can use LLM APIs, extractive methods, TF-IDF, etc.
type Summarizer interface {
	// Summarize returns a summary for the given content.
	// Returns empty string if summarization is not available.
	Summarize(ctx context.Context, domain, path, content string) (string, error)
}

// NoOpSummarizer is the default — relies on agent-submitted summaries only.
type NoOpSummarizer struct{}

func (n *NoOpSummarizer) Summarize(ctx context.Context, domain, path, content string) (string, error) {
	return "", nil
}
