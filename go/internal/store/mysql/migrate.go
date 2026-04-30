package mysql

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

// delimiterRe matches a MySQL DELIMITER directive line:
//
//	DELIMITER $$   or   DELIMITER ;
var delimiterRe = regexp.MustCompile(`(?i)^\s*DELIMITER\s+(\S+)\s*$`)

// splitStatements splits MySQL SQL content into individual executable
// statements, handling the MySQL CLI DELIMITER directive. The DELIMITER
// lines themselves are consumed and not returned as statements.
//
// This is required for migration 3 which defines a BEFORE INSERT trigger
// using DELIMITER $$. The Go driver does not understand the DELIMITER
// directive — it is a mysql-CLI-only extension — so we must parse it here.
func splitStatements(content string) []string {
	delimiter := ";"
	var statements []string
	var cur strings.Builder

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Handle DELIMITER directive.
		if m := delimiterRe.FindStringSubmatch(trimmed); m != nil {
			// Flush any accumulated content before switching delimiter.
			if stmt := strings.TrimSpace(cur.String()); stmt != "" {
				statements = append(statements, stmt)
			}
			cur.Reset()
			delimiter = strings.TrimSpace(m[1])
			continue
		}

		cur.WriteString(line)
		cur.WriteString("\n")

		// A statement ends when the current line's trimmed form ends with
		// the current delimiter. This matches both:
		//   `CREATE TABLE ... ;` when delimiter is ";"
		//   `END$$`              when delimiter is "$$"
		if strings.HasSuffix(trimmed, delimiter) {
			stmt := strings.TrimRight(strings.TrimSpace(cur.String()), delimiter)
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				statements = append(statements, stmt)
			}
			cur.Reset()
		}
	}
	// Flush any trailing content not terminated by the delimiter.
	if stmt := strings.TrimSpace(cur.String()); stmt != "" {
		statements = append(statements, stmt)
	}
	return statements
}

// applyMigration executes all statements in a single migration file, then
// records the version in schema_migrations. MySQL DDL is non-transactional,
// so no outer transaction is used. The version is recorded after all
// statements succeed; on partial failure the next MigrateUp invocation will
// retry the migration from the beginning.
func applyMigration(db *sql.DB, v int, raw string) error {
	stmts := splitStatements(raw)
	for i, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration %d stmt %d: %w", v, i+1, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, v); err != nil {
		return fmt.Errorf("record migration %d: %w", v, err)
	}
	return nil
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
