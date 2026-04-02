package sqlite

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

var fkPragmaRe = regexp.MustCompile(`(?i)^\s*PRAGMA\s+foreign_keys\s*=`)

// splitFKPragmas removes PRAGMA foreign_keys lines from a migration body and
// returns whether any of them disable FK checks (OFF / 0). The cleaned body
// contains only non-PRAGMA-foreign_keys statements. This lets the migration
// runner execute those PRAGMAs at the connection level (they are no-ops
// inside a transaction per SQLite docs).
func splitFKPragmas(body string) (disableFK bool, cleaned string) {
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		if fkPragmaRe.MatchString(line) {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "off") || strings.Contains(lower, "= 0") || strings.Contains(lower, "=0") {
				disableFK = true
			}
			continue
		}
		kept = append(kept, line)
	}
	return disableFK, strings.Join(kept, "\n")
}

// applyMigration runs a single versioned migration inside a transaction.
// PRAGMA foreign_keys statements are extracted and executed at the connection
// level because they are no-ops inside a transaction in SQLite.
func applyMigration(db *sql.DB, v int, raw string) error {
	disableFK, body := splitFKPragmas(raw)
	if disableFK {
		if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
			return fmt.Errorf("disable fk for migration %d: %w", v, err)
		}
	}
	// Always restore FK enforcement after this migration, regardless of outcome.
	defer func() { _, _ = db.Exec("PRAGMA foreign_keys=ON") }()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(body); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec migration %d: %w", v, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, v); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %d: %w", v, err)
	}
	return tx.Commit()
}

// MigrateUp applies all pending *.up.sql migrations in dir (filenames like 000001_name.up.sql).
func MigrateUp(db *sql.DB, dir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER NOT NULL PRIMARY KEY
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
