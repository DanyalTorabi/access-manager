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

func TestMigrateUp_restoresForeignKeysAfterPRAGMAOffAndFailure(t *testing.T) {
	db, err := Open("file:" + filepath.Join(t.TempDir(), "mig.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dir := t.TempDir()
	// First statement disables FK checks; second fails. Migrator must leave enforcement ON afterward.
	body := "PRAGMA foreign_keys = OFF;\nNOT VALID SQL ;;;\n"
	if err := os.WriteFile(filepath.Join(dir, "000001_bad_fk.up.sql"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateUp(db, dir); err == nil {
		t.Fatal("want error for bad SQL")
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("want PRAGMA foreign_keys restored to 1 after failed migration, got %d", fk)
	}
}

func TestSplitFKPragmas(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOff   bool
		wantClean string
	}{
		{
			name:      "no pragmas",
			input:     "CREATE TABLE t (id TEXT);\nINSERT INTO t VALUES ('a');",
			wantOff:   false,
			wantClean: "CREATE TABLE t (id TEXT);\nINSERT INTO t VALUES ('a');",
		},
		{
			name:      "off and on stripped",
			input:     "PRAGMA foreign_keys = OFF;\nDROP TABLE t;\nPRAGMA foreign_keys = ON;",
			wantOff:   true,
			wantClean: "DROP TABLE t;",
		},
		{
			name:      "case insensitive",
			input:     "pragma Foreign_Keys = off;\nSELECT 1;\npragma foreign_keys=on;",
			wantOff:   true,
			wantClean: "SELECT 1;",
		},
		{
			name:      "only on not off",
			input:     "PRAGMA foreign_keys = ON;\nSELECT 1;",
			wantOff:   false,
			wantClean: "SELECT 1;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOff, gotClean := splitFKPragmas(tt.input)
			if gotOff != tt.wantOff {
				t.Errorf("disableFK = %v, want %v", gotOff, tt.wantOff)
			}
			if gotClean != tt.wantClean {
				t.Errorf("cleaned =\n%s\nwant\n%s", gotClean, tt.wantClean)
			}
		})
	}
}

func TestMigrateUp_fkOffAppliedOutsideTx(t *testing.T) {
	db, err := Open("file:" + filepath.Join(t.TempDir(), "mig.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dir := t.TempDir()
	// Migration uses PRAGMA foreign_keys=OFF then creates a table. The PRAGMA
	// must be applied at the connection level (outside the tx) by the runner.
	body := "PRAGMA foreign_keys = OFF;\nCREATE TABLE fk_test (id TEXT PRIMARY KEY);\nPRAGMA foreign_keys = ON;\n"
	if err := os.WriteFile(filepath.Join(dir, "000001_fk.up.sql"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateUp(db, dir); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("want PRAGMA foreign_keys restored to 1 after migration, got %d", fk)
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
