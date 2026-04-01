package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ensureSQLiteForeignKeysOn reapplies FK enforcement on the connection pool.
// Migrations (e.g. table rebuilds) may run PRAGMA foreign_keys=OFF; if a migration
// fails mid-script, restore ON so the process does not keep running with FKs disabled.
func ensureSQLiteForeignKeysOn(db *sql.DB) {
	_, _ = db.Exec("PRAGMA foreign_keys=ON")
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
		path := files[v]
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %d: %w", v, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			ensureSQLiteForeignKeysOn(db)
			return fmt.Errorf("exec migration %d: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, v); err != nil {
			_ = tx.Rollback()
			ensureSQLiteForeignKeysOn(db)
			return fmt.Errorf("record migration %d: %w", v, err)
		}
		if err := tx.Commit(); err != nil {
			ensureSQLiteForeignKeysOn(db)
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
