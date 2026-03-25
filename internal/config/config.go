package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envConfigPath        = "CONFIG_PATH"
	envDatabaseDriver    = "DATABASE_DRIVER"
	envDatabaseURL       = "DATABASE_URL"
	envHTTPAddr          = "HTTP_ADDR"
	envMigrationsDir     = "MIGRATIONS_DIR"
	envShutdownTimeoutSec = "SHUTDOWN_TIMEOUT_SECONDS"
)

// fileShape matches config.example.yaml (snake_case keys).
type fileShape struct {
	DatabaseDriver          string `yaml:"database_driver"`
	DatabaseURL             string `yaml:"database_url"`
	HTTPAddr                string `yaml:"http_addr"`
	MigrationsDir           string `yaml:"migrations_dir"`
	ShutdownTimeoutSeconds  *int   `yaml:"shutdown_timeout_seconds"`
}

// Config is resolved runtime configuration after defaults, optional file, and env overrides.
type Config struct {
	DatabaseDriver   string
	DatabaseURL      string
	HTTPAddr         string
	MigrationsDir    string
	ShutdownTimeout  time.Duration
}

// Load builds configuration: defaults → optional YAML file (CONFIG_PATH) → environment overrides.
// Env always wins when set to a non-empty value. If CONFIG_PATH is unset, file is skipped (env-only / defaults).
func Load() (Config, error) {
	c := Config{
		DatabaseDriver:  "sqlite",
		DatabaseURL:     "file:access.db?_pragma=foreign_keys(1)",
		HTTPAddr:        "127.0.0.1:8080",
		MigrationsDir:   "migrations/sqlite",
		ShutdownTimeout: 30 * time.Second,
	}

	path := strings.TrimSpace(os.Getenv(envConfigPath))
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("config: read %s: %w", path, err)
		}
		var f fileShape
		if err := yaml.Unmarshal(b, &f); err != nil {
			return Config{}, fmt.Errorf("config: yaml: %w", err)
		}
		if f.DatabaseDriver != "" {
			c.DatabaseDriver = f.DatabaseDriver
		}
		if f.DatabaseURL != "" {
			c.DatabaseURL = f.DatabaseURL
		}
		if f.HTTPAddr != "" {
			c.HTTPAddr = f.HTTPAddr
		}
		if f.MigrationsDir != "" {
			c.MigrationsDir = f.MigrationsDir
		}
		if f.ShutdownTimeoutSeconds != nil && *f.ShutdownTimeoutSeconds > 0 {
			c.ShutdownTimeout = time.Duration(*f.ShutdownTimeoutSeconds) * time.Second
		}
	}

	if v := os.Getenv(envDatabaseDriver); v != "" {
		c.DatabaseDriver = v
	}
	if v := os.Getenv(envDatabaseURL); v != "" {
		c.DatabaseURL = v
	}
	if v := os.Getenv(envHTTPAddr); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv(envMigrationsDir); v != "" {
		c.MigrationsDir = v
	}
	if v := strings.TrimSpace(os.Getenv(envShutdownTimeoutSec)); v != "" {
		sec, err := strconv.Atoi(v)
		if err != nil || sec <= 0 {
			return Config{}, fmt.Errorf("config: %s must be a positive integer (seconds)", envShutdownTimeoutSec)
		}
		c.ShutdownTimeout = time.Duration(sec) * time.Second
	}

	if err := validate(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func validate(c Config) error {
	if strings.TrimSpace(c.DatabaseDriver) == "" {
		return errors.New("config: database_driver is required")
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("config: database_url is required")
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return errors.New("config: http_addr is required")
	}
	if strings.TrimSpace(c.MigrationsDir) == "" {
		return errors.New("config: migrations_dir is required")
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("config: shutdown timeout must be positive")
	}
	return nil
}
