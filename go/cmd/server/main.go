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
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

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

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())

	st := sqlstore.New(db)
	srv := &api.Server{Store: st, APIBearerToken: cfg.APIBearerToken}

	httpSrv := &http.Server{
		Handler:           srv.Router(reg, reg),
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
		logger.Warn(msg)
	}
}
