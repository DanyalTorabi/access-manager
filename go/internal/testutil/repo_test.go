package testutil

import (
	"os"
	"path/filepath"
	"strings"
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

func TestPostgresMigrationsDir(t *testing.T) {
	dir := PostgresMigrationsDir(t)
	if !strings.HasSuffix(filepath.ToSlash(dir), "migrations/postgres") {
		t.Fatalf("PostgresMigrationsDir() = %q, want path ending in migrations/postgres", dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("PostgresMigrationsDir() = %q: %v", dir, err)
	}
}

func TestMySQLMigrationsDir(t *testing.T) {
	dir := MySQLMigrationsDir(t)
	if !strings.HasSuffix(filepath.ToSlash(dir), "migrations/mysql") {
		t.Fatalf("MySQLMigrationsDir() = %q, want path ending in migrations/mysql", dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("MySQLMigrationsDir() = %q: %v", dir, err)
	}
}
