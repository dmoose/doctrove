package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "llmshadow.yaml"

type Config struct {
	Sites map[string]*SiteConfig `yaml:"sites"`
	path  string
}

type SiteConfig struct {
	URL        string    `yaml:"url"`
	Include    []string  `yaml:"include,omitempty"`
	Exclude    []string  `yaml:"exclude,omitempty"`
	UpdateFreq string    `yaml:"update_freq,omitempty"`
	LastSync   time.Time `yaml:"last_sync,omitempty"`
}

// Load reads the config from the given root directory.
// If the file doesn't exist, returns an empty config.
func Load(rootDir string) (*Config, error) {
	path := filepath.Join(rootDir, DefaultConfigFile)
	cfg := &Config{
		Sites: make(map[string]*SiteConfig),
		path:  path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
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
	cfg.path = path
	return cfg, nil
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

// UpdateLastSync records the sync time for a site.
func (c *Config) UpdateLastSync(domain string, t time.Time) {
	if site, ok := c.Sites[domain]; ok {
		site.LastSync = t
	}
}
