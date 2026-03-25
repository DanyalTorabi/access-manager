package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dtorabi/access-manager/internal/api"
	"github.com/dtorabi/access-manager/internal/database"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
)

func main() {
	driver := getenv("DATABASE_DRIVER", "sqlite")
	dsn := getenv("DATABASE_URL", "file:access.db?_pragma=foreign_keys(1)")
	addr := getenv("HTTP_ADDR", "127.0.0.1:8080")
	migDir := getenv("MIGRATIONS_DIR", "migrations/sqlite")

	db, _, err := database.Open(driver, dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if !filepath.IsAbs(migDir) {
		if wd, err := os.Getwd(); err == nil {
			migDir = filepath.Join(wd, migDir)
		}
	}
	if err := database.MigrateUp(db, migDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	st := sqlstore.New(db)
	srv := &api.Server{Store: st}

	log.Printf("listening on http://%s", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
