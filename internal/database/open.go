package database

import (
	"database/sql"
	"fmt"

	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
)

// Open returns a *sql.DB for the given driver and DSN, plus the migrations directory
// for that dialect (relative to the process working directory unless overridden).
//
// Supported drivers: "sqlite", "sqlite3" (modernc.org/sqlite). PostgreSQL and MySQL
// can be added here with matching store and migration implementations.
func Open(driver, dsn string) (*sql.DB, string, error) {
	switch driver {
	case "sqlite", "sqlite3":
		db, err := sqlstore.Open(dsn)
		if err != nil {
			return nil, "", err
		}
		return db, "migrations/sqlite", nil
	default:
		return nil, "", fmt.Errorf("database: unsupported driver %q (use sqlite or sqlite3)", driver)
	}
}

// MigrateUp applies SQL migrations in dir using the SQLite migrator (file-based .up.sql).
// Other dialects should use their own migrator when implemented.
func MigrateUp(db *sql.DB, migrationsDir string) error {
	return sqlstore.MigrateUp(db, migrationsDir)
}
