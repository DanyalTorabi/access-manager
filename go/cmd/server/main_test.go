package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dtorabi/access-manager/internal/config"
	"github.com/dtorabi/access-manager/internal/testutil"
)

func TestMaybeWarnAPIAuth_loopback(t *testing.T) {
	maybeWarnAPIAuth(config.Config{HTTPAddr: "127.0.0.1:8080"})
}

func TestMaybeWarnAPIAuth_nonLoopbackNoToken(t *testing.T) {
	maybeWarnAPIAuth(config.Config{HTTPAddr: "0.0.0.0:8080"})
}

func TestMaybeWarnAPIAuth_withToken(t *testing.T) {
	maybeWarnAPIAuth(config.Config{HTTPAddr: "0.0.0.0:8080", APIBearerToken: "secret"})
}

func testCfg(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		DatabaseDriver:  "sqlite",
		DatabaseURL:     "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)",
		HTTPAddr:        "127.0.0.1:0",
		MigrationsDir:   filepath.Join(testutil.RepoRoot(t), "migrations", "sqlite"),
		ShutdownTimeout: 5 * time.Second,
	}
}

// --- setup tests ---

func TestSetup_success(t *testing.T) {
	cfg := testCfg(t)
	httpSrv, db, err := setup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if httpSrv == nil {
		t.Fatal("nil http server")
	}
}

func TestSetup_badDriver(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver:  "unknown",
		DatabaseURL:     "noop",
		HTTPAddr:        "127.0.0.1:0",
		MigrationsDir:   "migrations/sqlite",
		ShutdownTimeout: 5 * time.Second,
	}
	if _, _, err := setup(cfg); err == nil {
		t.Fatal("want error for unsupported driver")
	}
}

func TestSetup_badMigrations(t *testing.T) {
	cfg := testCfg(t)
	cfg.MigrationsDir = filepath.Join(t.TempDir(), "nonexistent-migrations")
	if _, _, err := setup(cfg); err == nil {
		t.Fatal("want error for bad migrations dir")
	}
}

// --- run tests ---

func TestRun_success(t *testing.T) {
	cfg := testCfg(t)

	// Grab the port run() will listen on so we can poll readiness.
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	cfg.HTTPAddr = addr

	stop := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- run(cfg, stop)
	}()

	pollHealth(t, "http://"+addr+"/health")
	stop <- syscall.SIGINT

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after signal")
	}
}

func TestRun_badDriver(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver:  "unknown",
		DatabaseURL:     "noop",
		HTTPAddr:        "127.0.0.1:0",
		MigrationsDir:   "migrations/sqlite",
		ShutdownTimeout: 5 * time.Second,
	}
	stop := make(chan os.Signal, 1)
	if err := run(cfg, stop); err == nil {
		t.Fatal("want error for bad driver")
	}
}

func TestRun_badListenAddr(t *testing.T) {
	cfg := testCfg(t)
	cfg.HTTPAddr = "999.999.999.999:0"
	stop := make(chan os.Signal, 1)
	if err := run(cfg, stop); err == nil {
		t.Fatal("want error for bad listen address")
	}
}

// --- serve tests ---

func TestServe_cleanShutdown(t *testing.T) {
	cfg := testCfg(t)
	httpSrv, db, err := setup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	stop := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- serve(httpSrv, ln, 5*time.Second, stop)
	}()

	pollHealth(t, "http://"+ln.Addr().String()+"/health")

	stop <- syscall.SIGINT

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after signal")
	}
}

func TestServe_listenerError(t *testing.T) {
	httpSrv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()

	stop := make(chan os.Signal, 1)
	if err := serve(httpSrv, ln, 5*time.Second, stop); err == nil {
		t.Fatal("want error for closed listener")
	}
}

// --- runMain tests ---

func TestRunMain_badConfig(t *testing.T) {
	clearCfgEnv(t)
	t.Setenv("DATABASE_DRIVER", " ")
	if err := runMain(); err == nil {
		t.Fatal("want error for invalid config")
	}
}

func clearCfgEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"CONFIG_PATH", "DATABASE_DRIVER", "DATABASE_URL",
		"HTTP_ADDR", "MIGRATIONS_DIR", "SHUTDOWN_TIMEOUT_SECONDS",
		"API_BEARER_TOKEN",
	} {
		t.Setenv(k, "")
	}
}

// pollHealth retries GET url until 200 OK or 3 s deadline, with per-request timeout.
func pollHealth(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server not ready within 3s: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
