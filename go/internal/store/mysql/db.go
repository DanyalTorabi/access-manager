package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // register "mysql" driver
)

// DriverName is the database/sql driver name registered by go-sql-driver/mysql.
const DriverName = "mysql"

// Open opens a MySQL database connection and verifies it with Ping.
// The DSN must include parseTime=true. FK checks are explicitly enabled
// after opening in case the DSN connects to a session where they were
// previously disabled.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS=1"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set foreign_key_checks: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
