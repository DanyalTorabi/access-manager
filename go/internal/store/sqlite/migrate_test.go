package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dtorabi/access-manager/internal/testutil"
)

func TestMigrateUp_success(t *testing.T) {
	db, err := Open("file:" + filepath.Join(t.TempDir(), "mig.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dir := testutil.SQLiteMigrationsDir(t)
	if err := MigrateUp(db, dir); err != nil {
		t.Fatal(err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v < 1 {
		t.Fatalf("want version >= 1, got %d", v)
	}
}

func TestMigrateUp_idempotent(t *testing.T) {
	db, err := Open("file:" + filepath.Join(t.TempDir(), "mig.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dir := testutil.SQLiteMigrationsDir(t)
	if err := MigrateUp(db, dir); err != nil {
		t.Fatal(err)
	}
	if err := MigrateUp(db, dir); err != nil {
		t.Fatalf("second run should be idempotent: %v", err)
	}
}

func TestMigrateUp_badDir(t *testing.T) {
	db, err := Open("file:" + filepath.Join(t.TempDir(), "mig.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := MigrateUp(db, filepath.Join(t.TempDir(), "nonexistent-mig-dir")); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestMigrateUp_badSQL(t *testing.T) {
	db, err := Open("file:" + filepath.Join(t.TempDir(), "mig.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001_bad.up.sql"), []byte("NOT VALID SQL ;;;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateUp(db, dir); err == nil {
		t.Fatal("want error for bad SQL")
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		wantV   int
		wantOK  bool
	}{
		{"000001_init.up.sql", 1, true},
		{"000023_add_col.up.sql", 23, true},
		{"noversion.up.sql", 0, false},
		{"abc_foo.up.sql", 0, false},
	}
	for _, tt := range tests {
		v, ok := parseVersion(tt.name)
		if ok != tt.wantOK || v != tt.wantV {
			t.Errorf("parseVersion(%q) = %d, %v; want %d, %v", tt.name, v, ok, tt.wantV, tt.wantOK)
		}
	}
}
