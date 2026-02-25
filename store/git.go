package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitStore wraps git operations on the workspace repository.
type GitStore struct {
	root string
	repo *git.Repository
}

// InitGit initializes or opens the git repository in the workspace root.
// On a fresh workspace it creates a seed commit so HEAD is valid immediately.
func InitGit(root string) (*GitStore, error) {
	gs := &GitStore{root: root}

	repo, err := git.PlainOpen(root)
	if err == nil {
		gs.repo = repo
		// Recover from a partial init (e.g. PlainOpen found .git but no HEAD).
		if err := gs.ensureHead(); err != nil {
			return nil, err
		}
		return gs, nil
	}

	// Not a repo yet — initialize.
	repo, err = git.PlainInit(root, false)
	if err != nil {
		return nil, fmt.Errorf("initializing git repo: %w", err)
	}
	gs.repo = repo

	// Create initial .gitignore for the workspace.
	gitignore := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		content := "doctrove.db\ndoctrove.db-wal\ndoctrove.db-shm\n"
		if err := os.WriteFile(gitignore, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("writing .gitignore: %w", err)
		}
	}

	// Seed commit so HEAD is valid from the start.
	if err := gs.seedCommit(); err != nil {
		return nil, fmt.Errorf("creating seed commit: %w", err)
	}

	return gs, nil
}

// seedCommit stages .gitignore and creates the initial commit.
func (gs *GitStore) seedCommit() error {
	wt, err := gs.repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := wt.Add(".gitignore"); err != nil {
		return err
	}
	_, err = wt.Commit("init workspace", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "doctrove",
			Email: "doctrove@local",
			When:  time.Now(),
		},
	})
	return err
}

// ensureHead checks that HEAD resolves to a valid ref. If the repo was
// partially initialized (e.g. has .git/objects but no HEAD), it recovers
// by creating a seed commit.
func (gs *GitStore) ensureHead() error {
	_, err := gs.repo.Head()
	if err == nil {
		return nil
	}
	// HEAD is dangling — stage whatever exists and commit.
	wt, err := gs.repo.Worktree()
	if err != nil {
		return fmt.Errorf("recovering HEAD: %w", err)
	}
	_ = wt.AddGlob(".")
	_, err = wt.Commit("recover workspace", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "doctrove",
			Email: "doctrove@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("recovering HEAD: %w", err)
	}
	return nil
}

// Commit stages all changes and creates a commit. Returns true if a commit was
// created, false if there was nothing to commit.
func (gs *GitStore) Commit(message string) (bool, error) {
	wt, err := gs.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("getting worktree: %w", err)
	}

	// Stage all changes
	if err := wt.AddGlob("."); err != nil {
		return false, fmt.Errorf("staging changes: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("checking status: %w", err)
	}

	if status.IsClean() {
		return false, nil
	}

	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "doctrove",
			Email: "doctrove@local",
			When:  time.Now(),
		},
	})
	if err != nil {
		return false, fmt.Errorf("committing: %w", err)
	}

	return true, nil
}

// Log returns recent commit entries, optionally filtered to a site's path.
type LogEntry struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	When    time.Time `json:"when"`
}

func (gs *GitStore) Log(site string, limit int) ([]LogEntry, error) {
	opts := &git.LogOptions{}
	if site != "" {
		// Filter to the site's directory
		opts.PathFilter = func(path string) bool {
			return strings.HasPrefix(path, "sites/"+site+"/")
		}
	}

	iter, err := gs.repo.Log(opts)
	if err != nil {
		return nil, fmt.Errorf("reading log: %w", err)
	}
	defer iter.Close()

	var entries []LogEntry
	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && len(entries) >= limit {
			return fmt.Errorf("limit reached")
		}
		entries = append(entries, LogEntry{
			Hash:    c.Hash.String()[:10],
			Message: strings.Split(c.Message, "\n")[0],
			When:    c.Author.When,
		})
		return nil
	})
	// "limit reached" is our own sentinel, not a real error
	if err != nil && err.Error() != "limit reached" {
		return nil, err
	}

	return entries, nil
}

// Diff returns the diff between two refs (e.g., "HEAD~1", "HEAD").
// If from is empty, defaults to the parent of to.
func (gs *GitStore) Diff(from, to string) (string, error) {
	toCommit, err := gs.resolveCommit(to)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", to, err)
	}

	var fromCommit *object.Commit
	if from == "" {
		// Use parent of to
		parents := toCommit.Parents()
		fromCommit, err = parents.Next()
		if err != nil {
			// No parent — first commit, diff against empty tree
			toTree, err := toCommit.Tree()
			if err != nil {
				return "", fmt.Errorf("getting tree: %w", err)
			}
			patch, err := (&object.Tree{}).Patch(toTree)
			if err != nil {
				return "", fmt.Errorf("generating patch: %w", err)
			}
			return patch.String(), nil
		}
	} else {
		fromCommit, err = gs.resolveCommit(from)
		if err != nil {
			return "", fmt.Errorf("resolving %q: %w", from, err)
		}
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("getting from tree: %w", err)
	}
	toTree, err := toCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("getting to tree: %w", err)
	}

	patch, err := fromTree.Patch(toTree)
	if err != nil {
		return "", fmt.Errorf("generating diff: %w", err)
	}
	return patch.String(), nil
}

// HasChanges returns true if the worktree has uncommitted changes.
func (gs *GitStore) HasChanges() (bool, error) {
	wt, err := gs.repo.Worktree()
	if err != nil {
		return false, err
	}
	status, err := wt.Status()
	if err != nil {
		return false, err
	}
	return !status.IsClean(), nil
}

func (gs *GitStore) resolveCommit(ref string) (*object.Commit, error) {
	hash, err := gs.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, err
	}
	return gs.repo.CommitObject(*hash)
}
