package engine

import (
	"fmt"

	"github.com/dmoose/doctrove/internal/config"
	"github.com/dmoose/doctrove/internal/content"
	"github.com/dmoose/doctrove/internal/discovery"
	"github.com/dmoose/doctrove/internal/events"
	"github.com/dmoose/doctrove/internal/fetcher"
	"github.com/dmoose/doctrove/internal/mirror"
	"github.com/dmoose/doctrove/internal/robots"
	"github.com/dmoose/doctrove/internal/store"
)

// Engine is the core orchestrator that ties all subsystems together.
type Engine struct {
	Config      *config.Config
	Store       *store.Store
	Git         *store.GitStore
	Index       store.Indexer
	Discovery   *discovery.Discoverer
	Mirror      *mirror.Mirror
	Fetcher     *fetcher.Fetcher
	Events      *events.Emitter
	Processors  []content.Processor
	Categorizer store.Categorizer
	RootDir     string
}

// Options configures engine behavior.
type Options struct {
	RespectRobots bool
}

// New creates an Engine rooted at the given directory.
func New(rootDir string, opts ...Options) (*Engine, error) {
	cfg, err := config.Load(rootDir)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	f := fetcher.New(fetcher.Options{
		UserAgent:    cfg.Settings.UserAgent,
		RatePerHost:  cfg.Settings.RateLimit,
		BurstPerHost: cfg.Settings.RateBurst,
		Timeout:      cfg.Settings.TimeoutDuration(),
	})
	s := store.New(rootDir)

	gs, err := store.InitGit(rootDir)
	if err != nil {
		return nil, fmt.Errorf("initializing git: %w", err)
	}

	idx, err := store.OpenIndex(rootDir)
	if err != nil {
		return nil, fmt.Errorf("opening search index: %w", err)
	}

	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	var rc *robots.Checker
	if o.RespectRobots {
		rc = robots.New(f)
	}

	em := events.New(cfg.Settings.EventsURL, "doctrove")

	disc := discovery.New(f, rc, cfg.Settings.MaxProbes)

	// Register additional providers based on config
	if cfg.Settings.Context7APIKey != "" {
		disc.RegisterProvider(discovery.NewContext7Provider(cfg.Settings.Context7APIKey))
	}

	return &Engine{
		Config:      cfg,
		Store:       s,
		Git:         gs,
		Index:       idx,
		Discovery:   disc,
		Mirror:      mirror.New(f, s, idx),
		Fetcher:     f,
		Events:      em,
		Processors:  []content.Processor{&content.MarkdownProcessor{}},
		Categorizer: &store.RuleCategorizer{},
		RootDir:     rootDir,
	}, nil
}

// Close releases resources held by the engine.
func (e *Engine) Close() error {
	e.Events.Flush()
	return e.Index.Close()
}
