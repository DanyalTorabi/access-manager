package main

import (
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dtorabi/access-manager/internal/config"
	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/testutil"
)

func captureLog(fn func()) string {
	var buf bytes.Buffer
	logger.Init(slog.LevelDebug, &buf)
	defer logger.Init(slog.LevelInfo, os.Stderr)
	fn()
	return buf.String()
}

func TestMaybeWarnAPIAuth_loopback(t *testing.T) {
	out := captureLog(func() {
		maybeWarnAPIAuth(config.Config{HTTPAddr: "127.0.0.1:8080"})
	})
	if out != "" {
		t.Fatalf("loopback should not warn, got: %s", out)
	}
}

func TestMaybeWarnAPIAuth_nonLoopbackNoToken(t *testing.T) {
	out := captureLog(func() {
		maybeWarnAPIAuth(config.Config{HTTPAddr: "0.0.0.0:8080"})
	})
	if !strings.Contains(out, "API_BEARER_TOKEN") {
		t.Fatalf("non-loopback without token should warn, got: %q", out)
	}
}

func TestMaybeWarnAPIAuth_withToken(t *testing.T) {
	out := captureLog(func() {
		maybeWarnAPIAuth(config.Config{HTTPAddr: "0.0.0.0:8080", APIBearerToken: "test-token"})
	})
	if out != "" {
		t.Fatalf("with token should not warn, got: %s", out)
	}
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	stop := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- run(cfg, ln, stop)
	}()

	pollHealth(t, "http://"+ln.Addr().String()+"/health")
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
	if err := run(cfg, nil, stop); err == nil {
		t.Fatal("want error for bad driver")
	}
}

func TestRun_badListenAddr(t *testing.T) {
	cfg := testCfg(t)
	cfg.HTTPAddr = "999.999.999.999:0"
	stop := make(chan os.Signal, 1)
	if err := run(cfg, nil, stop); err == nil {
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
	httpSrv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
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
	var lastErr error
	var lastStatus int
	for {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
		} else {
			lastStatus = resp.StatusCode
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = nil
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				t.Fatalf("server not ready within 3s: %v", lastErr)
			}
			t.Fatalf("server not ready within 3s: last status %d", lastStatus)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
