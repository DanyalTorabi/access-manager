package testutil

import (
	"path/filepath"
	"runtime"
	"testing"
)

// RepoRoot returns the repository root (directory containing go.mod).
func RepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/testutil -> repo root is two levels up from this file's directory
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// SQLiteMigrationsDir returns migrations/sqlite under the repo root.
func SQLiteMigrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(RepoRoot(t), "migrations", "sqlite")
}
