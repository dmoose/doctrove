package robots

import (
	"bufio"
	"context"
	"strings"
	"sync"

	"github.com/dmoose/doctrove/fetcher"
)

// AI-specific user-agents to check in robots.txt.
var aiUserAgents = []string{
	"doctrove",
	"GPTBot",
	"ClaudeBot",
	"anthropic-ai",
	"ChatGPT-User",
	"Google-Extended",
	"CCBot",
	"PerplexityBot",
	"Bytespider",
	"Cohere-ai",
}

// Checker evaluates robots.txt rules for AI user-agents.
type Checker struct {
	fetcher *fetcher.Fetcher
	cache   map[string]*robotsRules
	mu      sync.Mutex
}

// New creates a Checker.
func New(f *fetcher.Fetcher) *Checker {
	return &Checker{
		fetcher: f,
		cache:   make(map[string]*robotsRules),
	}
}

// IsAllowed checks whether the given URL is allowed by robots.txt.
// Returns true if allowed (or if robots.txt is absent/unparseable).
func (c *Checker) IsAllowed(ctx context.Context, rawURL string) bool {
	host := extractHost(rawURL)
	rules := c.getRules(ctx, host)
	if rules == nil {
		return true
	}

	path := extractPath(rawURL)

	// Check AI-specific user-agents first
	for _, ua := range aiUserAgents {
		if block, ok := rules.agents[strings.ToLower(ua)]; ok {
			return block.isAllowed(path)
		}
	}

	// Fall back to wildcard
	if block, ok := rules.agents["*"]; ok {
		return block.isAllowed(path)
	}

	return true
}

func (c *Checker) getRules(ctx context.Context, host string) *robotsRules {
	c.mu.Lock()
	if rules, ok := c.cache[host]; ok {
		c.mu.Unlock()
		return rules
	}
	c.mu.Unlock()

	// Fetch and parse
	url := "https://" + host + "/robots.txt"
	resp, err := c.fetcher.Fetch(ctx, url)
	if err != nil || resp == nil {
		// Try http
		url = "http://" + host + "/robots.txt"
		resp, err = c.fetcher.Fetch(ctx, url)
	}

	var rules *robotsRules
	if err == nil && resp != nil {
		rules = parseRobots(string(resp.Body))
	}

	c.mu.Lock()
	c.cache[host] = rules
	c.mu.Unlock()
	return rules
}

type robotsRules struct {
	agents map[string]*agentBlock // lowercase user-agent -> rules
}

type agentBlock struct {
	disallow []string
	allow    []string
}

func (b *agentBlock) isAllowed(path string) bool {
	// Check allow rules first (more specific wins, but we use simple first-match)
	for _, pattern := range b.allow {
		if matchPath(pattern, path) {
			return true
		}
	}
	for _, pattern := range b.disallow {
		if matchPath(pattern, path) {
			return false
		}
	}
	return true
}

func parseRobots(body string) *robotsRules {
	rules := &robotsRules{agents: make(map[string]*agentBlock)}
	var currentAgents []string

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Strip comments
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent":
			ua := strings.ToLower(val)
			// If we hit a new user-agent after directives, start fresh
			if len(currentAgents) > 0 {
				// Check if previous agents had any rules
				if _, ok := rules.agents[currentAgents[0]]; ok {
					currentAgents = nil
				}
			}
			currentAgents = append(currentAgents, ua)
			if _, ok := rules.agents[ua]; !ok {
				rules.agents[ua] = &agentBlock{}
			}
		case "disallow":
			for _, ua := range currentAgents {
				rules.agents[ua].disallow = append(rules.agents[ua].disallow, val)
			}
		case "allow":
			for _, ua := range currentAgents {
				rules.agents[ua].allow = append(rules.agents[ua].allow, val)
			}
		}
	}
	return rules
}

// matchPath checks if a robots.txt pattern matches a URL path.
// Supports prefix matching, * wildcards, and $ end anchor.
func matchPath(pattern, path string) bool {
	if pattern == "" {
		return false
	}

	// Handle $ end anchor
	if strings.HasSuffix(pattern, "$") {
		pattern = pattern[:len(pattern)-1]
		if !strings.Contains(pattern, "*") {
			return path == pattern
		}
	}

	// Handle * wildcards
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		pos := 0
		for i, part := range parts {
			if part == "" {
				continue
			}
			idx := strings.Index(path[pos:], part)
			if idx < 0 {
				return false
			}
			if i == 0 && idx != 0 {
				// First part must match from start if no leading *
				return false
			}
			pos += idx + len(part)
		}
		return true
	}

	// Simple prefix match
	return strings.HasPrefix(path, pattern)
}

func extractHost(rawURL string) string {
	start := 0
	if i := strings.Index(rawURL, "://"); i >= 0 {
		start = i + 3
	}
	end := len(rawURL)
	if i := strings.IndexByte(rawURL[start:], '/'); i >= 0 {
		end = start + i
	}
	return rawURL[start:end]
}

func extractPath(rawURL string) string {
	start := 0
	if i := strings.Index(rawURL, "://"); i >= 0 {
		start = i + 3
	}
	if i := strings.IndexByte(rawURL[start:], '/'); i >= 0 {
		return rawURL[start+i:]
	}
	return "/"
}
