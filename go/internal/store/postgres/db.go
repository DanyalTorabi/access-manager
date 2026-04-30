package postgres

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // register "postgres" driver
)

// DriverName is the database/sql driver name registered by lib/pq.
const DriverName = "postgres"

// Open opens a PostgreSQL database connection and verifies it with Ping.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
