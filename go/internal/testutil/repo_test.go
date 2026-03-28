package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoRoot(t *testing.T) {
	root := RepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("RepoRoot() = %q, go.mod not found: %v", root, err)
	}
}

func TestSQLiteMigrationsDir(t *testing.T) {
	dir := SQLiteMigrationsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("SQLiteMigrationsDir() = %q: %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatal("migrations directory is empty")
	}
}
