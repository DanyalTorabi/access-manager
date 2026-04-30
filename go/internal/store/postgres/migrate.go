package postgres

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// beginRe matches a standalone BEGIN; line (case-insensitive, optional whitespace).
var beginRe = regexp.MustCompile(`(?im)^\s*BEGIN\s*;\s*$`)

// commitRe matches a standalone COMMIT; line (case-insensitive, optional whitespace).
var commitRe = regexp.MustCompile(`(?im)^\s*COMMIT\s*;\s*$`)

// stripTransactionMarkers removes standalone BEGIN; and COMMIT; lines from a
// migration body so that the runner can wrap the SQL in its own transaction
// together with the version-recording INSERT.
func stripTransactionMarkers(body string) string {
	body = beginRe.ReplaceAllString(body, "")
	body = commitRe.ReplaceAllString(body, "")
	return strings.TrimSpace(body)
}

// applyMigration runs a single versioned migration inside a transaction.
// The migration body may contain multiple semicolon-separated statements.
// BEGIN/COMMIT markers from the file are stripped; atomicity is provided by
// this function's own transaction, which also records the version number.
func applyMigration(db *sql.DB, v int, raw string) error {
	body := stripTransactionMarkers(raw)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(body); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec migration %d: %w", v, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, v); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %d: %w", v, err)
	}
	return tx.Commit()
}

// MigrateUp applies all pending *.up.sql migrations in dir (filenames like 000001_name.up.sql).
func MigrateUp(db *sql.DB, dir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT NOT NULL PRIMARY KEY
	)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	var cur int
	row := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&cur); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var versions []int
	files := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		v, ok := parseVersion(e.Name())
		if !ok {
			continue
		}
		versions = append(versions, v)
		files[v] = filepath.Join(dir, e.Name())
	}
	sort.Ints(versions)

	for _, v := range versions {
		if v <= cur {
			continue
		}
		body, err := os.ReadFile(files[v])
		if err != nil {
			return fmt.Errorf("read migration %d: %w", v, err)
		}
		if err := applyMigration(db, v, string(body)); err != nil {
			return err
		}
	}
	return nil
}

func parseVersion(name string) (int, bool) {
	base := filepath.Base(name)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) < 2 {
		return 0, false
	}
	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	return v, true
}
