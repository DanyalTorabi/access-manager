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
	_, err := Open("file:/dev/null/nonexistent/path.db")
	if err == nil {
		t.Fatal("want error for invalid DSN")
	}
}
