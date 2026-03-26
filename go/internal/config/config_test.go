package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_defaultsOnly(t *testing.T) {
	t.Setenv(envConfigPath, "")
	t.Setenv(envDatabaseDriver, "")
	t.Setenv(envDatabaseURL, "")
	t.Setenv(envHTTPAddr, "")
	t.Setenv(envMigrationsDir, "")
	t.Setenv(envShutdownTimeoutSec, "")
	t.Setenv(envAPIBearerToken, "")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseDriver != "sqlite" {
		t.Fatalf("driver: %q", c.DatabaseDriver)
	}
	if c.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("addr: %q", c.HTTPAddr)
	}
	if c.ShutdownTimeout.Seconds() != 30 {
		t.Fatalf("shutdown: %v", c.ShutdownTimeout)
	}
	if c.APIBearerToken != "" {
		t.Fatalf("api bearer: %q", c.APIBearerToken)
	}
}

func TestLoad_envOverrides(t *testing.T) {
	t.Setenv(envConfigPath, "")
	t.Setenv(envDatabaseDriver, "sqlite")
	t.Setenv(envDatabaseURL, "file::memory:")
	t.Setenv(envHTTPAddr, "127.0.0.1:9999")
	t.Setenv(envMigrationsDir, "migrations/sqlite")
	t.Setenv(envShutdownTimeoutSec, "5")
	t.Setenv(envAPIBearerToken, "env-bearer")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != "127.0.0.1:9999" {
		t.Fatalf("addr: %q", c.HTTPAddr)
	}
	if c.ShutdownTimeout.Seconds() != 5 {
		t.Fatalf("shutdown: %v", c.ShutdownTimeout)
	}
	if c.APIBearerToken != "env-bearer" {
		t.Fatalf("api bearer: %q", c.APIBearerToken)
	}
}

func TestLoad_fromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
database_driver: sqlite
database_url: "file:custom.db?_pragma=foreign_keys(1)"
http_addr: "127.0.0.1:7000"
migrations_dir: migrations/sqlite
shutdown_timeout_seconds: 15
api_bearer_token: "from-yaml"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(envConfigPath, path)
	t.Setenv(envDatabaseDriver, "")
	t.Setenv(envDatabaseURL, "")
	t.Setenv(envHTTPAddr, "")
	t.Setenv(envMigrationsDir, "")
	t.Setenv(envShutdownTimeoutSec, "")
	t.Setenv(envAPIBearerToken, "")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != "127.0.0.1:7000" {
		t.Fatalf("addr: %q", c.HTTPAddr)
	}
	if c.DatabaseURL != "file:custom.db?_pragma=foreign_keys(1)" {
		t.Fatalf("dsn: %q", c.DatabaseURL)
	}
	if c.ShutdownTimeout.Seconds() != 15 {
		t.Fatalf("shutdown: %v", c.ShutdownTimeout)
	}
	if c.APIBearerToken != "from-yaml" {
		t.Fatalf("api bearer: %q", c.APIBearerToken)
	}
}

func TestLoad_envOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte(`http_addr: "127.0.0.1:7000"
api_bearer_token: "from-file"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfigPath, path)
	t.Setenv(envHTTPAddr, "127.0.0.1:8000")
	t.Setenv(envDatabaseDriver, "sqlite")
	t.Setenv(envDatabaseURL, "file::memory:")
	t.Setenv(envMigrationsDir, "migrations/sqlite")
	t.Setenv(envAPIBearerToken, "from-env")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != "127.0.0.1:8000" {
		t.Fatalf("env should override file, got %q", c.HTTPAddr)
	}
	if c.APIBearerToken != "from-env" {
		t.Fatalf("api bearer: env should override file, got %q", c.APIBearerToken)
	}
}
