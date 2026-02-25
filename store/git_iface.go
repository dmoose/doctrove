package store

// VersionStore provides git-based change tracking for mirrored content.
// The default implementation uses go-git. Alternative implementations could
// use shell git, a different VCS, or a no-op for environments without git.
type VersionStore interface {
	Commit(message string) (bool, error)
	Log(site string, limit int) ([]LogEntry, error)
	Diff(from, to string) (string, error)
	HasChanges() (bool, error)
}

// Verify that GitStore satisfies VersionStore at compile time.
var _ VersionStore = (*GitStore)(nil)
