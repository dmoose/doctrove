package engine

import (
	"fmt"

	"github.com/dmoose/doctrove/config"
	"github.com/dmoose/doctrove/content"
	"github.com/dmoose/doctrove/discovery"
	"github.com/dmoose/doctrove/events"
	"github.com/dmoose/doctrove/fetcher"
	"github.com/dmoose/doctrove/mirror"
	"github.com/dmoose/doctrove/internal/robots"
	"github.com/dmoose/doctrove/store"
)

// Engine is the core orchestrator that ties all subsystems together.
// All component fields are interfaces, allowing alternative implementations
// to be injected via functional options in New().
type Engine struct {
	Config      *config.Config
	Store       *store.Store
	Git         store.VersionStore
	Index       store.Indexer
	Discovery   discovery.ContentDiscoverer
	Mirror      mirror.Syncer
	Fetcher     fetcher.HTTPFetcher
	Events      events.EventEmitter
	Processors  []content.Processor
	Categorizer store.Categorizer
	RootDir     string
}

// Option configures engine behavior via functional options.
type Option func(*engineOpts)

type engineOpts struct {
	respectRobots bool
	fetcher       fetcher.HTTPFetcher
	syncer        mirror.Syncer
	discovery     discovery.ContentDiscoverer
	git           store.VersionStore
	events        events.EventEmitter
	indexer       store.Indexer
	processors    []content.Processor
	categorizer   store.Categorizer
	config        *config.Config
}

// WithRespectRobots enables robots.txt checking during discovery.
func WithRespectRobots() Option {
	return func(o *engineOpts) { o.respectRobots = true }
}

// WithFetcher injects a custom HTTP fetcher (e.g. Playwright-capable).
func WithFetcher(f fetcher.HTTPFetcher) Option {
	return func(o *engineOpts) { o.fetcher = f }
}

// WithSyncer injects a custom content syncer (e.g. with cleaning/chunking pipeline).
func WithSyncer(s mirror.Syncer) Option {
	return func(o *engineOpts) { o.syncer = s }
}

// WithDiscovery injects a custom content discoverer.
func WithDiscovery(d discovery.ContentDiscoverer) Option {
	return func(o *engineOpts) { o.discovery = d }
}

// WithGit injects a custom version store for change tracking.
func WithGit(g store.VersionStore) Option {
	return func(o *engineOpts) { o.git = g }
}

// WithEvents injects a custom event emitter for observability.
func WithEvents(e events.EventEmitter) Option {
	return func(o *engineOpts) { o.events = e }
}

// WithIndexer injects a custom search indexer (e.g. Bleve, vector search).
func WithIndexer(i store.Indexer) Option {
	return func(o *engineOpts) { o.indexer = i }
}

// WithProcessors sets the content processors (e.g. markdown, reST, chunker).
func WithProcessors(p ...content.Processor) Option {
	return func(o *engineOpts) { o.processors = p }
}

// WithCategorizer injects a custom page categorizer (e.g. LLM-based).
func WithCategorizer(c store.Categorizer) Option {
	return func(o *engineOpts) { o.categorizer = c }
}

// WithConfig injects a pre-loaded configuration, skipping file-based loading.
func WithConfig(c *config.Config) Option {
	return func(o *engineOpts) { o.config = c }
}

// New creates an Engine rooted at the given directory.
// Components not provided via options are constructed with defaults.
func New(rootDir string, opts ...Option) (*Engine, error) {
	var o engineOpts
	for _, opt := range opts {
		opt(&o)
	}

	// Config
	cfg := o.config
	if cfg == nil {
		var err error
		cfg, err = config.Load(rootDir)
		if err != nil {
			return nil, fmt.Errorf("loading config: %w", err)
		}
	}

	// Store (filesystem layout — always default, not pluggable)
	s := store.New(rootDir)

	// Git
	var git store.VersionStore = o.git
	if git == nil {
		gs, err := store.InitGit(rootDir)
		if err != nil {
			return nil, fmt.Errorf("initializing git: %w", err)
		}
		git = gs
	}

	// Indexer
	var idx store.Indexer = o.indexer
	if idx == nil {
		i, err := store.OpenIndex(rootDir)
		if err != nil {
			return nil, fmt.Errorf("opening search index: %w", err)
		}
		idx = i
	}

	// Fetcher
	var f fetcher.HTTPFetcher = o.fetcher
	if f == nil {
		f = fetcher.New(fetcher.Options{
			UserAgent:    cfg.Settings.UserAgent,
			RatePerHost:  cfg.Settings.RateLimit,
			BurstPerHost: cfg.Settings.RateBurst,
			Timeout:      cfg.Settings.TimeoutDuration(),
		})
	}

	// Events
	var em events.EventEmitter = o.events
	if em == nil {
		em = events.New(cfg.Settings.EventsURL, "doctrove")
	}

	// Discovery
	var disc discovery.ContentDiscoverer = o.discovery
	if disc == nil {
		// robots.Checker needs the concrete fetcher type for the default path
		var rc *robots.Checker
		if o.respectRobots {
			if cf, ok := f.(*fetcher.Fetcher); ok {
				rc = robots.New(cf)
			}
		}
		d := discovery.New(f, rc, cfg.Settings.MaxProbes)
		if cfg.Settings.HasContext7Key() {
			d.RegisterProvider(discovery.NewContext7Provider(cfg.Settings.Context7APIKey))
		}
		disc = d
	}

	// Mirror / Syncer
	var syn mirror.Syncer = o.syncer
	if syn == nil {
		syn = mirror.New(f, s, idx)
	}

	// Processors
	processors := o.processors
	if len(processors) == 0 {
		processors = []content.Processor{&content.MarkdownProcessor{}}
	}

	// Categorizer
	var cat store.Categorizer = o.categorizer
	if cat == nil {
		cat = &store.RuleCategorizer{}
	}

	return &Engine{
		Config:      cfg,
		Store:       s,
		Git:         git,
		Index:       idx,
		Discovery:   disc,
		Mirror:      syn,
		Fetcher:     f,
		Events:      em,
		Processors:  processors,
		Categorizer: cat,
		RootDir:     rootDir,
	}, nil
}

// Close releases resources held by the engine.
func (e *Engine) Close() error {
	e.Events.Flush()
	return e.Index.Close()
}
