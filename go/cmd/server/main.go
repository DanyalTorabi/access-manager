package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	mysqlstore "github.com/dtorabi/access-manager/internal/store/mysql"
	pgstore "github.com/dtorabi/access-manager/internal/store/postgres"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// maskHookSetter is satisfied by all three concrete store types (sqlite, postgres, mysql).
// It allows the negative-mask Prometheus hook to be wired without coupling main to any
// one store implementation.
type maskHookSetter interface {
	SetNegativeMaskHook(func())
}

func main() {
	logger.Init(slog.LevelInfo, os.Stderr)
	if err := runMain(); err != nil {
		logger.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func runMain() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	return run(cfg, nil, sigCh)
}

// run wires setup → listen → serve. If ln is non-nil it is used directly
// (useful for tests that need a deterministic port); otherwise a new
// listener is created from cfg.HTTPAddr.
func run(cfg config.Config, ln net.Listener, stop <-chan os.Signal) error {
	httpSrv, db, err := setup(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if ln == nil {
		ln, err = net.Listen("tcp", cfg.HTTPAddr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
		}
	}

	logger.Info("listening", slog.String("addr", "http://"+ln.Addr().String()))
	return serve(httpSrv, ln, cfg.ShutdownTimeout, stop)
}

// serve starts the HTTP server on ln and blocks until a signal arrives on stop,
// then gracefully shuts down within timeout.
func serve(httpSrv *http.Server, ln net.Listener, timeout time.Duration, stop <-chan os.Signal) error {
	errCh := make(chan error, 1)
	go func() {
		err := httpSrv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
		logger.Info("shutting down", slog.String("signal", sig.String()))
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
	logger.Info("server stopped")
	return nil
}

// setup wires config → DB → migrations → HTTP server and returns the server and DB handle.
func setup(cfg config.Config) (*http.Server, *sql.DB, error) {
	maybeWarnAPIAuth(cfg)
	maybeWarnCORS(cfg)

	db, migDirFromDriver, err := database.Open(cfg.DatabaseDriver, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	// Use cfg.MigrationsDir as the authoritative source, but auto-correct the common
	// misconfiguration where DATABASE_DRIVER is changed without updating MIGRATIONS_DIR.
	// If the migrations dir still has the compile-time sqlite default but the selected
	// driver is not sqlite, fall back to the driver-canonical path and log a warning.
	migDir := cfg.MigrationsDir
	if migDir == "migrations/sqlite" && migDirFromDriver != "migrations/sqlite" {
		logger.Warn("MIGRATIONS_DIR not updated from default; using driver-canonical path",
			slog.String("driver", cfg.DatabaseDriver),
			slog.String("migrations_dir", migDirFromDriver),
		)
		migDir = migDirFromDriver
	}
	if !filepath.IsAbs(migDir) {
		if wd, err := os.Getwd(); err == nil {
			migDir = filepath.Join(wd, migDir)
		}
	}
	if err := database.MigrateUp(db, migDir, cfg.DatabaseDriver); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())

	var st store.Store
	switch cfg.DatabaseDriver {
	case "postgres":
		st = pgstore.New(db)
	case "mysql":
		st = mysqlstore.New(db)
	case "sqlite", "sqlite3":
		st = sqlstore.New(db)
	default:
		// database.Open already rejected unsupported drivers; this case is unreachable in practice.
		_ = db.Close()
		return nil, nil, fmt.Errorf("setup: unrecognized driver %q", cfg.DatabaseDriver)
	}

	srv := &api.Server{Store: st, APIBearerToken: cfg.APIBearerToken, CORSAllowedOrigins: cfg.CORSAllowedOrigins}

	httpSrv := &http.Server{
		Handler:           srv.Router(reg, reg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Wire the negative-mask observer to the Prometheus counter so operators
	// can alert on legacy/out-of-band data. All three store implementations
	// satisfy maskHookSetter. See T50.
	if c := srv.NegativeMaskCounter(); c != nil {
		if hs, ok := st.(maskHookSetter); ok {
			hs.SetNegativeMaskHook(c.Inc)
		} else {
			logger.Warn("store does not implement SetNegativeMaskHook; store_negative_mask_observed_total will not increment")
		}
	}

	// Backfill / repair the user_resource_masks materialized cache. For
	// Postgres and MySQL this is a no-op; for SQLite it rebuilds from the
	// source tables so any pre-T04 data (or drift from out-of-band writes)
	// is corrected before the server starts accepting traffic.
	// A 5-minute timeout guards against unexpectedly large datasets blocking
	// startup indefinitely.
	reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer reconcileCancel()
	if err := st.ReconcileUserResourceMasks(reconcileCtx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("reconcile user_resource_masks: %w", err)
	}
	logger.Info("user_resource_masks reconciled")

	return httpSrv, db, nil
}

// maybeWarnAPIAuth logs once if the API may be reachable beyond loopback without Bearer protection.
func maybeWarnAPIAuth(cfg config.Config) {
	if msg := config.APIAuthStartupWarning(cfg); msg != "" {
		logger.Warn(msg)
	}
}

// maybeWarnCORS logs once if CORS is configured with a wildcard origin on a non-loopback address.
func maybeWarnCORS(cfg config.Config) {
	if msg := config.CORSStartupWarning(cfg); msg != "" {
		logger.Warn(msg)
	}
}
