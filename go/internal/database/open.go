package database

import (
	"database/sql"
	"fmt"
	"time"

	// Store packages are imported here only for their MigrateUp functions.
	// TODO(T-new): extract dialect migration runners into this package so that
	// internal/database no longer depends on internal/store/* (CML-T60-6).
	mysqlstore "github.com/dtorabi/access-manager/internal/store/mysql"
	pgstore "github.com/dtorabi/access-manager/internal/store/postgres"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
)

// Open opens a *sql.DB for the given driver and DSN, plus the canonical migrations
// directory for that dialect. The driver must be pre-registered before calling Open;
// importing any of the store packages (internal/store/{sqlite,postgres,mysql}) as a
// blank import registers the corresponding database/sql driver as a side-effect.
//
// Supported drivers: "sqlite" / "sqlite3" (modernc.org/sqlite), "postgres" (lib/pq),
// "mysql" (go-sql-driver/mysql).
func Open(driver, dsn string) (*sql.DB, string, error) {
	switch driver {
	case "sqlite", "sqlite3":
		db, err := sql.Open(sqlstore.DriverName, dsn)
		if err != nil {
			return nil, "", err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("sqlite ping: %w", err)
		}
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("pragma foreign_keys: %w", err)
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		return db, "migrations/sqlite", nil
	case "postgres":
		db, err := sql.Open(pgstore.DriverName, dsn)
		if err != nil {
			return nil, "", err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("postgres ping: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
		return db, "migrations/postgres", nil
	case "mysql":
		db, err := sql.Open(mysqlstore.DriverName, dsn)
		if err != nil {
			return nil, "", err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("mysql ping: %w", err)
		}
		if _, err := db.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
			_ = db.Close()
			return nil, "", fmt.Errorf("set foreign_key_checks: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
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
