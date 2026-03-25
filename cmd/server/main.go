package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dtorabi/access-manager/internal/api"
	"github.com/dtorabi/access-manager/internal/config"
	"github.com/dtorabi/access-manager/internal/database"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, _, err := database.Open(cfg.DatabaseDriver, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	migDir := cfg.MigrationsDir
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

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("listening on http://%s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := <-sigCh
	log.Printf("signal received: %v, shutting down", sig)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Printf("server stopped")
}
