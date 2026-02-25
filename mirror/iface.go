package mirror

import (
	"context"

	"github.com/dmoose/doctrove/discovery"
)

// Syncer downloads discovered content and writes it to the local store.
// The default implementation fetches via HTTP, converts HTML to markdown,
// rewrites links, and compares content for change detection. Alternative
// implementations can add content cleaning, chunking, or custom pipelines.
type Syncer interface {
	Sync(ctx context.Context, result *discovery.Result, filter FilterFunc) (*SyncResult, error)
}

// Verify that Mirror satisfies Syncer at compile time.
var _ Syncer = (*Mirror)(nil)
