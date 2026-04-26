package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoad_apiBearerTokenYAMLTrimmed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
database_driver: sqlite
database_url: "file::memory"
http_addr: "127.0.0.1:8080"
migrations_dir: migrations/sqlite
api_bearer_token: "  yaml-token  "
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
	if c.APIBearerToken != "yaml-token" {
		t.Fatalf("api bearer should be trimmed from YAML, got %q", c.APIBearerToken)
	}
}

func TestLoad_apiBearerTokenYAMLWhitespaceOnlySkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
database_driver: sqlite
database_url: "file::memory:"
http_addr: "127.0.0.1:8080"
migrations_dir: migrations/sqlite
api_bearer_token: "  	  "
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
	if c.APIBearerToken != "" {
		t.Fatalf("whitespace-only YAML api_bearer_token should not apply, got %q", c.APIBearerToken)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{envConfigPath, envDatabaseDriver, envDatabaseURL, envHTTPAddr, envMigrationsDir, envShutdownTimeoutSec, envAPIBearerToken} {
		t.Setenv(k, "")
	}
}

func TestLoad_invalidShutdownTimeout(t *testing.T) {
	clearEnv(t)
	t.Setenv(envShutdownTimeoutSec, "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("want error for non-integer shutdown timeout")
	}
	if !strings.Contains(err.Error(), envShutdownTimeoutSec) {
		t.Fatalf("error should mention %s, got: %v", envShutdownTimeoutSec, err)
	}
}

func TestLoad_negativeShutdownTimeout(t *testing.T) {
	clearEnv(t)
	t.Setenv(envShutdownTimeoutSec, "-1")
	_, err := Load()
	if err == nil {
		t.Fatal("want error for negative shutdown timeout")
	}
	if !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("error should mention 'positive integer', got: %v", err)
	}
}

func TestLoad_missingConfigFile(t *testing.T) {
	clearEnv(t)
	t.Setenv(envConfigPath, filepath.Join(t.TempDir(), "nonexistent-cfg.yaml"))
	_, err := Load()
	if err == nil {
		t.Fatal("want error for missing config file")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Fatalf("error should mention config, got: %v", err)
	}
}

func TestLoad_invalidYAML(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(":::not yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfigPath, path)
	_, err := Load()
	if err == nil {
		t.Fatal("want error for invalid yaml")
	}
}

func TestValidate_emptyDriver(t *testing.T) {
	clearEnv(t)
	t.Setenv(envDatabaseDriver, " ")
	_, err := Load()
	if err == nil {
		t.Fatal("want error for empty driver")
	}
	if !strings.Contains(err.Error(), "database_driver") {
		t.Fatalf("error should mention database_driver, got: %v", err)
	}
}

func TestValidate_emptyDatabaseURL(t *testing.T) {
	clearEnv(t)
	t.Setenv(envDatabaseURL, " ")
	_, err := Load()
	if err == nil {
		t.Fatal("want error for empty database url")
	}
	if !strings.Contains(err.Error(), "database_url") {
		t.Fatalf("error should mention database_url, got: %v", err)
	}
}

func TestValidate_emptyHTTPAddr(t *testing.T) {
	clearEnv(t)
	t.Setenv(envHTTPAddr, " ")
	_, err := Load()
	if err == nil {
		t.Fatal("want error for empty http addr")
	}
	if !strings.Contains(err.Error(), "http_addr") {
		t.Fatalf("error should mention http_addr, got: %v", err)
	}
}

func TestValidate_emptyMigrationsDir(t *testing.T) {
	clearEnv(t)
	t.Setenv(envMigrationsDir, " ")
	_, err := Load()
	if err == nil {
		t.Fatal("want error for empty migrations dir")
	}
	if !strings.Contains(err.Error(), "migrations_dir") {
		t.Fatalf("error should mention migrations_dir, got: %v", err)
	}
}
