package database

import (
	"database/sql"
	"fmt"

	mysqlstore "github.com/dtorabi/access-manager/internal/store/mysql"
	pgstore "github.com/dtorabi/access-manager/internal/store/postgres"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
)

// Open returns a *sql.DB for the given driver and DSN, plus the migrations directory
// for that dialect (relative to the process working directory unless overridden).
//
// Supported drivers: "sqlite" / "sqlite3" (modernc.org/sqlite), "postgres" (lib/pq),
// "mysql" (go-sql-driver/mysql).
func Open(driver, dsn string) (*sql.DB, string, error) {
	switch driver {
	case "sqlite", "sqlite3":
		db, err := sqlstore.Open(dsn)
		if err != nil {
			return nil, "", err
		}
		return db, "migrations/sqlite", nil
	case "postgres":
		db, err := pgstore.Open(dsn)
		if err != nil {
			return nil, "", err
		}
		return db, "migrations/postgres", nil
	case "mysql":
		db, err := mysqlstore.Open(dsn)
		if err != nil {
			return nil, "", err
		}
		return db, "migrations/mysql", nil
	default:
		return nil, "", fmt.Errorf("database: unsupported driver %q (supported: sqlite, postgres, mysql)", driver)
	}
}

// MigrateUp applies SQL migrations in dir using the migrator for the given driver.
func MigrateUp(db *sql.DB, migrationsDir, driver string) error {
	switch driver {
	case "sqlite", "sqlite3":
		return sqlstore.MigrateUp(db, migrationsDir)
	case "postgres":
		return pgstore.MigrateUp(db, migrationsDir)
	case "mysql":
		return mysqlstore.MigrateUp(db, migrationsDir)
	default:
		return fmt.Errorf("database: unsupported driver %q (supported: sqlite, postgres, mysql)", driver)
	}
}
