package main

import (
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/dtorabi/access-manager/internal/config"
	"github.com/dtorabi/access-manager/internal/testutil"
)

func TestMaybeWarnAPIAuth_loopback(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:       "127.0.0.1:8080",
		APIBearerToken: "",
	}
	maybeWarnAPIAuth(cfg)
}

func TestMaybeWarnAPIAuth_nonLoopbackNoToken(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:       "0.0.0.0:8080",
		APIBearerToken: "",
	}
	maybeWarnAPIAuth(cfg)
}

func TestMaybeWarnAPIAuth_withToken(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:       "0.0.0.0:8080",
		APIBearerToken: "secret",
	}
	maybeWarnAPIAuth(cfg)
}

func TestSetup_success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := config.Config{
		DatabaseDriver:  "sqlite",
		DatabaseURL:     "file:" + dbPath + "?_pragma=foreign_keys(1)",
		HTTPAddr:        "127.0.0.1:0",
		MigrationsDir:   filepath.Join(testutil.RepoRoot(t), "migrations", "sqlite"),
		ShutdownTimeout: 5 * time.Second,
	}
	httpSrv, db, err := setup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if httpSrv == nil {
		t.Fatal("nil http server")
	}
	if httpSrv.Addr != "127.0.0.1:0" {
		t.Fatalf("addr = %q", httpSrv.Addr)
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
	_, _, err := setup(cfg)
	if err == nil {
		t.Fatal("want error for unsupported driver")
	}
}

func TestSetup_badMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := config.Config{
		DatabaseDriver:  "sqlite",
		DatabaseURL:     "file:" + dbPath + "?_pragma=foreign_keys(1)",
		HTTPAddr:        "127.0.0.1:0",
		MigrationsDir:   filepath.Join(t.TempDir(), "nonexistent-migrations"),
		ShutdownTimeout: 5 * time.Second,
	}
	_, _, err := setup(cfg)
	if err == nil {
		t.Fatal("want error for bad migrations dir")
	}
}

func TestSetup_healthEndpoint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := config.Config{
		DatabaseDriver:  "sqlite",
		DatabaseURL:     "file:" + dbPath + "?_pragma=foreign_keys(1)",
		HTTPAddr:        "127.0.0.1:0",
		MigrationsDir:   filepath.Join(testutil.RepoRoot(t), "migrations", "sqlite"),
		ShutdownTimeout: 5 * time.Second,
	}
	httpSrv, db, err := setup(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() { _ = httpSrv.Close() })

	resp, err := http.Get("http://" + ln.Addr().String() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", resp.StatusCode)
	}
}
