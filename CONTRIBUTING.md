# Contributing to doctrove

## Development Setup

```bash
git clone https://github.com/dmoose/doctrove.git
cd doctrove
make build    # Build the binary
make test     # Run tests with race detector
```

### Prerequisites

- Go 1.26+

## Making Changes

1. Fork the repo and create a feature branch from `main`
2. Make your changes
3. Run `make fmt` and `make lint`
4. Run `make test` to ensure all tests pass
5. Submit a pull request

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Wrap errors with context: `fmt.Errorf("doing something: %w", err)`
- Write table-driven tests where appropriate
- Use `t.TempDir()` for tests that need filesystem access
- All public interfaces live in their own `_iface.go` files

## Architecture

Library-first: `engine` is the primary API surface with all dependencies injected via functional options. CLI and MCP are thin wrappers. See `DESIGN.md`.

Key conventions:
- `engine/` orchestrates everything
- `store/` owns filesystem layout, git, and SQLite index
- `discovery/` is provider-based; new content sources implement `Provider`
- `mirror/` handles the fetch-and-store pipeline

## What to Contribute

- Bug fixes with test cases
- New discovery providers (doc aggregators, package registries)
- Categorizer improvements (new path patterns, better heuristics)
- Content processor plugins
- Documentation improvements
- Performance improvements with benchmarks

## Reporting Issues

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Go version and OS

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
