# CLI Skills

How to use doctrove from the command line for scripting, debugging, and manual workflows.

## Setup

```bash
make install                    # build and install to $GOBIN
make init-workspace             # create ~/.config/doctrove with default config
```

Override workspace location with `--dir /path/to/workspace` or `DOCTROVE_DIR` env var.

All commands support `--json` for machine-readable output.

## Context7 (Enhanced Discovery)

With a Context7 API key configured, you can resolve bare library names to curated documentation:

```bash
doctrove grab react              # fetches Context7 docs for React
doctrove grab stripe-node        # fetches Context7 docs for Stripe Node SDK
```

Set `context7_api_key` in `doctrove.yaml`. Get a key at https://context7.com.

## Discovery & Ingestion

### Probe a site without tracking
```bash
doctrove discover https://stripe.com
```

### Add and sync in one step
```bash
doctrove grab https://stripe.com
```

### Add a site (probe only, no download)
```bash
doctrove init https://stripe.com
```

### Download content
```bash
doctrove sync stripe.com
doctrove sync --all              # sync all tracked sites
```

### Refresh with ETag caching
```bash
doctrove refresh stripe.com
doctrove refresh --all
```

## Search

### Full-text search
```bash
doctrove search "authentication"
doctrove search --site stripe.com "webhooks"
doctrove search --category api-reference "hooks"
doctrove search --full "webhook verification"   # returns full content of best match
```

### Rebuild search index
```bash
doctrove search --rebuild "test query"
```

## Browsing

### What's tracked
```bash
doctrove list                    # all tracked sites
doctrove catalog                 # sites with topics and categories
doctrove stats                   # disk usage, file counts, sync times
doctrove status stripe.com       # detail for one site
doctrove stale                   # sites not synced in 7 days
doctrove stale --threshold 3d    # custom threshold
```

### Dry-run: what would sync fetch
```bash
doctrove check stripe.com
```

## History & Changes

### Git log
```bash
doctrove history                 # all sites
doctrove history stripe.com      # one site
doctrove history --since 7d      # recent changes
```

### Diff between syncs
```bash
doctrove diff                    # last change
doctrove diff HEAD~3 HEAD        # range
doctrove diff --since 2h         # all changes in last 2 hours
doctrove diff --since 1d --stat  # last day, compact summary
```

## Management

### Override a category
```bash
doctrove tag stripe.com /payments guide
```

### Remove a site
```bash
doctrove remove stripe.com
doctrove remove --keep-files stripe.com   # remove from config, keep files
```

## MCP Server

### Start (stdio transport)
```bash
doctrove mcp
```

### Show config for your agent
```bash
doctrove mcp-config
```

## Configuration

Edit `~/.config/doctrove/doctrove.yaml`:

```yaml
settings:
  rate_limit: 2            # req/sec per host
  rate_burst: 5            # burst capacity
  timeout: 30s
  max_probes: 100          # companion probes per llms.txt
  user_agent: "doctrove/0.1"
  context7_api_key: ""     # optional Context7 API key

sites:
  stripe.com:
    url: https://stripe.com
    include:
      - "/llms*"
      - "/docs/**/*.md"
    exclude:
      - "/internal/**"
```

## Scripting Examples

### Refresh all stale sites
```bash
doctrove stale --json | jq -r '.[].domain' | while read site; do
  echo "Refreshing $site..."
  doctrove refresh "$site"
done
```

### Export search results as JSON
```bash
doctrove search --json --category api-reference "authentication" | jq '.results[].path'
```

### Check if a site has been synced today
```bash
doctrove status --json stripe.com | jq -r '.age'
```
