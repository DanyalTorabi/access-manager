package database

import (
	"path/filepath"
	"testing"

	"github.com/dtorabi/access-manager/internal/testutil"
)

func TestOpen_sqlite(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, migDir, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if migDir != "migrations/sqlite" {
		t.Fatalf("migDir = %q", migDir)
	}
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_sqlite3Alias(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, _, err := Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
}

func TestOpen_unsupportedDriver(t *testing.T) {
	_, _, err := Open("mongo", "localhost")
	if err == nil {
		t.Fatal("want error for unsupported driver")
	}
}

func TestOpen_sqlite_invalidDSN(t *testing.T) {
	_, _, err := Open("sqlite", "file:/nonexistent/path/db.sqlite?_pragma=foreign_keys(1)")
	if err == nil {
		t.Fatal("want error for invalid/inaccessible DSN")
	}
}

func TestMigrateUp_sqlite(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, _, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt == 0 {
		t.Fatal("migrations not applied")
	}
}
