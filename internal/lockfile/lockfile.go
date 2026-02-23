// Package lockfile provides file-based locking for workspace write operations.
package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const lockFileName = ".doctrove.lock"

// Lock holds an exclusive file lock on the workspace.
type Lock struct {
	file *os.File
}

// Acquire takes an exclusive lock on the workspace directory.
// Returns an error if another process holds the lock.
func Acquire(rootDir string) (*Lock, error) {
	path := filepath.Join(rootDir, lockFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("workspace is locked by another process")
	}

	// Write PID for debugging
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()

	return &Lock{file: f}, nil
}

// Release releases the lock.
func (l *Lock) Release() {
	if l.file != nil {
		_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		_ = l.file.Close()
		_ = os.Remove(l.file.Name())
	}
}
