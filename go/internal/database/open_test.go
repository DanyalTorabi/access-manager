package database

import (
	"path/filepath"
	"strings"
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

func TestOpen_postgres_pingError(t *testing.T) {
	_, _, err := Open("postgres", "postgres://invalid:invalid@127.0.0.1:1/nodb?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("want error for unreachable postgres DSN")
	}
}

func TestOpen_postgres_migrationsDir(t *testing.T) {
	// Verify the routing returns the correct migrations dir before the ping fails.
	// We check the error message is a ping error (not an "unsupported driver" error).
	_, migDir, err := Open("postgres", "postgres://invalid:invalid@127.0.0.1:1/nodb?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Skip("unexpected successful postgres connection")
	}
	// migDir is empty on error — but we verify the driver was dispatched (no "unsupported driver" in error).
	if migDir != "" {
		t.Fatalf("migDir should be empty on error, got %q", migDir)
	}
	if strings.Contains(err.Error(), "unsupported driver") {
		t.Fatalf("should not be unsupported driver error, got: %v", err)
	}
}

func TestOpen_mysql_pingError(t *testing.T) {
	_, _, err := Open("mysql", "invalid:invalid@tcp(127.0.0.1:1)/nodb?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s")
	if err == nil {
		t.Fatal("want error for unreachable mysql DSN")
	}
}

func TestOpen_mysql_migrationsDir(t *testing.T) {
	// Verify routing is reached (no "unsupported driver" error).
	_, migDir, err := Open("mysql", "invalid:invalid@tcp(127.0.0.1:1)/nodb?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s")
	if err == nil {
		t.Skip("unexpected successful mysql connection")
	}
	if migDir != "" {
		t.Fatalf("migDir should be empty on error, got %q", migDir)
	}
	if strings.Contains(err.Error(), "unsupported driver") {
		t.Fatalf("should not be unsupported driver error, got: %v", err)
	}
}

func TestOpen_unsupportedDriver(t *testing.T) {
	_, _, err := Open("mongo", "localhost")
	if err == nil {
		t.Fatal("want error for unsupported driver")
	}
	// Error should list supported drivers.
	for _, d := range []string{"sqlite", "postgres", "mysql"} {
		if !strings.Contains(err.Error(), d) {
			t.Errorf("error %q does not mention supported driver %q", err.Error(), d)
		}
	}
}

func TestOpen_sqlite_invalidDSN(t *testing.T) {
	dsn := "file:" + t.TempDir() + "?_pragma=foreign_keys(1)"
	_, _, err := Open("sqlite", dsn)
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
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t), "sqlite"); err != nil {
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

func TestMigrateUp_sqlite3Alias(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, _, err := Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t), "sqlite3"); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateUp_unsupportedDriver(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, _, err := Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	err = MigrateUp(db, testutil.SQLiteMigrationsDir(t), "baddriver")
	if err == nil {
		t.Fatal("want error for unsupported driver in MigrateUp")
	}
	if !strings.Contains(err.Error(), "unsupported driver") {
		t.Errorf("error %q should mention 'unsupported driver'", err.Error())
	}
}
