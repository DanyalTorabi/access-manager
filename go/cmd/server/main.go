package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	maybeWarnAPIAuth(cfg)

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
	srv := &api.Server{Store: st, APIBearerToken: cfg.APIBearerToken}

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

// maybeWarnAPIAuth logs once if the API may be reachable beyond loopback without Bearer protection.
func maybeWarnAPIAuth(cfg config.Config) {
	if strings.TrimSpace(cfg.APIBearerToken) != "" {
		return
	}
	addr := strings.TrimSpace(cfg.HTTPAddr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// e.g. ":8080" listens on all interfaces
		if strings.HasPrefix(addr, ":") {
			log.Printf("warning: API_BEARER_TOKEN is unset but HTTP_ADDR %q listens on all interfaces; set API_BEARER_TOKEN (or bind to 127.0.0.1) before exposing this service", addr)
		}
		return
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return
	}
	if strings.EqualFold(host, "localhost") {
		return
	}
	log.Printf("warning: API_BEARER_TOKEN is unset but http_addr host %q is not loopback; set API_BEARER_TOKEN (or bind to 127.0.0.1) before exposing this service", host)
}
