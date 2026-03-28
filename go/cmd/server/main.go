package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
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
	if err := runMain(); err != nil {
		log.Fatal(err)
	}
}

func runMain() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	return run(cfg, sigCh)
}

func run(cfg config.Config, stop <-chan os.Signal) error {
	httpSrv, db, err := setup(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}

	log.Printf("listening on http://%s", ln.Addr())
	return serve(httpSrv, ln, cfg.ShutdownTimeout, stop)
}

// serve starts the HTTP server on ln and blocks until a signal arrives on stop,
// then gracefully shuts down within timeout.
func serve(httpSrv *http.Server, ln net.Listener, timeout time.Duration, stop <-chan os.Signal) error {
	errCh := make(chan error, 1)
	go func() {
		err := httpSrv.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case sig := <-stop:
		log.Printf("signal received: %v, shutting down", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdownErr := httpSrv.Shutdown(ctx)

	// Block until Serve goroutine exits so we never miss an error.
	if err := <-errCh; err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	if shutdownErr != nil {
		return fmt.Errorf("shutdown: %w", shutdownErr)
	}
	log.Printf("server stopped")
	return nil
}

// setup wires config → DB → migrations → HTTP server and returns the server and DB handle.
func setup(cfg config.Config) (*http.Server, *sql.DB, error) {
	maybeWarnAPIAuth(cfg)

	db, _, err := database.Open(cfg.DatabaseDriver, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	migDir := cfg.MigrationsDir
	if !filepath.IsAbs(migDir) {
		if wd, err := os.Getwd(); err == nil {
			migDir = filepath.Join(wd, migDir)
		}
	}
	if err := database.MigrateUp(db, migDir); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
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
	return httpSrv, db, nil
}

// maybeWarnAPIAuth logs once if the API may be reachable beyond loopback without Bearer protection.
func maybeWarnAPIAuth(cfg config.Config) {
	if msg := config.APIAuthStartupWarning(cfg); msg != "" {
		log.Printf("warning: %s", msg)
	}
}
