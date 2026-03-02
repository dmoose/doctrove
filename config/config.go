package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "doctrove.yaml"

type Config struct {
	Settings *Settings               `yaml:"settings,omitempty"`
	Sites    map[string]*SiteConfig  `yaml:"sites"`
	path     string
}

type Settings struct {
	RateLimit      int    `yaml:"rate_limit,omitempty"`      // requests/sec per host
	RateBurst      int    `yaml:"rate_burst,omitempty"`      // burst capacity
	Timeout        string `yaml:"timeout,omitempty"`         // HTTP timeout (e.g. "30s")
	MaxProbes      int    `yaml:"max_probes,omitempty"`      // companion probes per llms.txt
	UserAgent      string `yaml:"user_agent,omitempty"`      // User-Agent header
	EventsURL      string `yaml:"events_url,omitempty"`      // URL for event relay (e.g. http://localhost:6060/events)
	Context7APIKey string `yaml:"context7_api_key"` // Context7 API key (get one at https://context7.com)
}

// DefaultSettings returns settings with sane defaults.
func DefaultSettings() *Settings {
	return &Settings{
		RateLimit: 2,
		RateBurst: 5,
		Timeout:   "30s",
		MaxProbes: 100,
		UserAgent: "doctrove/0.1",
	}
}

// HasContext7Key returns true if a valid Context7 API key is configured.
// Keys must start with the "ctx7sk" prefix to be considered valid.
func (s *Settings) HasContext7Key() bool {
	return strings.HasPrefix(s.Context7APIKey, "ctx7sk")
}

// TimeoutDuration parses the Timeout string as a time.Duration.
func (s *Settings) TimeoutDuration() time.Duration {
	d, err := time.ParseDuration(s.Timeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

type SiteConfig struct {
	URL          string    `yaml:"url"`
	Include      []string  `yaml:"include,omitempty"`
	Exclude      []string  `yaml:"exclude,omitempty"`
	ContentTypes string    `yaml:"content_types,omitempty"` // persisted from scan (e.g. "llms-txt,llms-full-txt")
	UpdateFreq   string    `yaml:"update_freq,omitempty"`
	LastSync     time.Time `yaml:"last_sync,omitempty"`
}

// Load reads the config from the given root directory.
// If the file doesn't exist, returns an empty config with default settings.
func Load(rootDir string) (*Config, error) {
	path := filepath.Join(rootDir, DefaultConfigFile)
	cfg := &Config{
		Sites: make(map[string]*SiteConfig),
		path:  path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.Settings = DefaultSettings()
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Sites == nil {
		cfg.Sites = make(map[string]*SiteConfig)
	}
	cfg.Settings = mergeSettings(cfg.Settings)
	cfg.path = path
	return cfg, nil
}

// mergeSettings fills zero-value fields from defaults.
func mergeSettings(s *Settings) *Settings {
	defaults := DefaultSettings()
	if s == nil {
		return defaults
	}
	if s.RateLimit <= 0 {
		s.RateLimit = defaults.RateLimit
	}
	if s.RateBurst <= 0 {
		s.RateBurst = defaults.RateBurst
	}
	if s.Timeout == "" {
		s.Timeout = defaults.Timeout
	}
	if s.MaxProbes <= 0 {
		s.MaxProbes = defaults.MaxProbes
	}
	if s.UserAgent == "" {
		s.UserAgent = defaults.UserAgent
	}
	return s
}

// Save writes the config back to disk.
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(c.path, data, 0644)
}

// AddSite adds a new site to the config. Returns an error if it already exists.
func (c *Config) AddSite(domain, url string) error {
	if _, exists := c.Sites[domain]; exists {
		return fmt.Errorf("site %q already tracked", domain)
	}
	c.Sites[domain] = &SiteConfig{
		URL: url,
	}
	return nil
}

// RemoveSite removes a site from the config.
func (c *Config) RemoveSite(domain string) error {
	if _, exists := c.Sites[domain]; !exists {
		return fmt.Errorf("site %q not tracked", domain)
	}
	delete(c.Sites, domain)
	return nil
}

// UpdateLastSync records the sync time for a site.
func (c *Config) UpdateLastSync(domain string, t time.Time) {
	if site, ok := c.Sites[domain]; ok {
		site.LastSync = t
	}
}

// SetContentTypes updates the content_types filter for a site, allowing
// agents to widen or narrow the filter after initial scan.
func (c *Config) SetContentTypes(domain, contentTypes string) {
	if site, ok := c.Sites[domain]; ok {
		site.ContentTypes = contentTypes
	}
}
