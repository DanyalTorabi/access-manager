package sqlite

import (
	"path/filepath"
	"testing"
)

func TestOpen_success(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "ok.db") + "?_pragma=foreign_keys(1)"
	db, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("want foreign_keys=1, got %d", fk)
	}
}

func TestOpen_invalidDSN(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "no-such-dir", "nested", "bad.db")
	_, err := Open(dsn)
	if err == nil {
		t.Fatal("want error for invalid DSN")
	}
}

// Read-only open against a non-existent file fails at Ping(), exercising the
// close + wrap error path in Open (not the PRAGMA path).
func TestOpen_readOnlyMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	dsn := "file:" + missing + "?mode=ro&_pragma=foreign_keys(1)"
	_, err := Open(dsn)
	if err == nil {
		t.Fatal("want error when read-only database file is missing")
	}
}
