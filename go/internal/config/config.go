package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envConfigPath         = "CONFIG_PATH"
	envDatabaseDriver     = "DATABASE_DRIVER"
	envDatabaseURL        = "DATABASE_URL"
	envHTTPAddr           = "HTTP_ADDR"
	envMigrationsDir      = "MIGRATIONS_DIR"
	envShutdownTimeoutSec = "SHUTDOWN_TIMEOUT_SECONDS"
	envAPIBearerToken     = "API_BEARER_TOKEN" // #nosec G101: environment variable name, not a hardcoded secret
	envCORSAllowedOrigins = "CORS_ALLOWED_ORIGINS"

	// corsSentinelDisable is a sentinel value that disables all CORS response headers.
	// Set CORS_ALLOWED_ORIGINS=none (or cors_allowed_origins: "none" in YAML) when a
	// reverse proxy manages CORS and in-process headers should be suppressed entirely.
	// The sentinel is case-insensitive; the resulting CORSAllowedOrigins slice is empty.
	corsSentinelDisable = "none"
)

// corsOriginList is a custom YAML type that accepts both a scalar comma-separated string
// and a proper YAML sequence, so operators can write either:
//
//	cors_allowed_origins: "https://a.com,https://b.com"   # comma-string (same as env var)
//	cors_allowed_origins:
//	  - https://a.com                                     # native YAML list
//	  - https://b.com
//
// When the scalar value is the sentinel "none" (case-insensitive, whitespace-trimmed),
// disabled is set to true so Load() can disable CORS without inspecting list contents
// for magic values. A YAML sequence containing "none" is NOT treated as a sentinel;
// it passes through to validate() which rejects it as an invalid origin URL.
type corsOriginList struct {
	origins  []string
	disabled bool
}

func (c *corsOriginList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var s []string
		if err := value.Decode(&s); err != nil {
			return err
		}
		c.origins = s
		c.disabled = false
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(s), corsSentinelDisable) {
			c.disabled = true
			c.origins = nil
			return nil
		}
		c.origins = parseCORSOrigins(s)
		c.disabled = false
	default:
		return fmt.Errorf("cors_allowed_origins: expected string or sequence, got %s", value.Tag)
	}
	return nil
}

// fileShape matches config.example.yaml (snake_case keys).
type fileShape struct {
	DatabaseDriver         string         `yaml:"database_driver"`
	DatabaseURL            string         `yaml:"database_url"`
	HTTPAddr               string         `yaml:"http_addr"`
	MigrationsDir          string         `yaml:"migrations_dir"`
	ShutdownTimeoutSeconds *int           `yaml:"shutdown_timeout_seconds"`
	APIBearerToken         string         `yaml:"api_bearer_token"`
	CORSAllowedOrigins     corsOriginList `yaml:"cors_allowed_origins"`
}

// Config is resolved runtime configuration after defaults, optional file, and env overrides.
type Config struct {
	DatabaseDriver  string
	DatabaseURL     string
	HTTPAddr        string
	MigrationsDir   string
	ShutdownTimeout time.Duration
	// APIBearerToken protects /api/v1/* when non-empty (Bearer scheme). Optional; see README.
	APIBearerToken string
	// CORSAllowedOrigins lists origins permitted in the Access-Control-Allow-Origin response header.
	// ["*"] (default) allows any origin. Empty slice disables CORS headers entirely.
	CORSAllowedOrigins []string
}

// Load builds configuration: defaults → optional YAML file (CONFIG_PATH) → environment overrides.
// Env always wins when set to a non-empty value. If CONFIG_PATH is unset, file is skipped (env-only / defaults).
func Load() (Config, error) {
	c := Config{
		DatabaseDriver:     "sqlite",
		DatabaseURL:        "file:access.db?_pragma=foreign_keys(1)",
		HTTPAddr:           "127.0.0.1:8080",
		MigrationsDir:      "migrations/sqlite",
		ShutdownTimeout:    30 * time.Second,
		CORSAllowedOrigins: []string{"*"},
	}

	path := strings.TrimSpace(os.Getenv(envConfigPath))
	if path != "" {
		// Reading configuration file at a path supplied by the environment is
		// expected (operator-provided). Validate presence but do not attempt
		// to enforce path restrictions here. Suppress gosec G304 as this is
		// an explicit config file read under operator control.
		b, err := os.ReadFile(path) // #nosec G304 G703 -- both rules apply to the same line: G304 (file inclusion) and G703 (taint via path arg); the path is operator-supplied via env and is intentionally unrestricted.
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
		if trimmed := strings.TrimSpace(f.APIBearerToken); trimmed != "" {
			c.APIBearerToken = trimmed
		}
		if f.CORSAllowedOrigins.disabled {
			c.CORSAllowedOrigins = []string{}
		} else if len(f.CORSAllowedOrigins.origins) > 0 {
			c.CORSAllowedOrigins = f.CORSAllowedOrigins.origins
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
	if v := strings.TrimSpace(os.Getenv(envAPIBearerToken)); v != "" {
		c.APIBearerToken = v
	}
	if v := strings.TrimSpace(os.Getenv(envCORSAllowedOrigins)); v != "" {
		if strings.EqualFold(v, corsSentinelDisable) {
			c.CORSAllowedOrigins = []string{}
		} else {
			parsed := parseCORSOrigins(v)
			if len(parsed) == 0 {
				return Config{}, fmt.Errorf("config: %s is set but contains no valid origin entries", envCORSAllowedOrigins)
			}
			c.CORSAllowedOrigins = parsed
		}
	}

	if err := validate(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// parseCORSOrigins splits a comma-separated origin string and trims each entry.
// It returns a non-empty slice; individual empty entries are dropped.
func parseCORSOrigins(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
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
	for _, origin := range c.CORSAllowedOrigins {
		if origin == "*" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("config: cors_allowed_origins: %q is not a valid origin (expected scheme://host[:port])", origin)
		}
	}
	return nil
}
