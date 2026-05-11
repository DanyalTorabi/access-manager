package config

import (
	"fmt"
	"net"
	"strings"
)

// APIAuthStartupWarning returns a non-empty message if the process should log at startup:
// the HTTP bind may expose /api beyond loopback while APIBearerToken is unset.
func APIAuthStartupWarning(c Config) string {
	if strings.TrimSpace(c.APIBearerToken) != "" {
		return ""
	}
	addr := strings.TrimSpace(c.HTTPAddr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	// Go accepts ":8080" as host "" + port; that listens on all interfaces.
	if host == "" {
		return fmt.Sprintf("API_BEARER_TOKEN is unset but HTTP_ADDR %q listens on all interfaces; set API_BEARER_TOKEN (or bind to 127.0.0.1) before exposing this service", addr)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return ""
	}
	if strings.EqualFold(host, "localhost") {
		return ""
	}
	return fmt.Sprintf("API_BEARER_TOKEN is unset but http_addr host %q is not loopback; set API_BEARER_TOKEN (or bind to 127.0.0.1) before exposing this service", host)
}

// CORSStartupWarning returns a non-empty message when CORS is configured with a
// wildcard origin ("*") and the server is not bound to a loopback address.
// A wildcard allows any browser origin, which may be unintentional in non-local environments.
func CORSStartupWarning(c Config) string {
	hasWildcard := false
	for _, o := range c.CORSAllowedOrigins {
		if o == "*" {
			hasWildcard = true
			break
		}
	}
	if !hasWildcard {
		return ""
	}
	addr := strings.TrimSpace(c.HTTPAddr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host == "" {
		return fmt.Sprintf("CORS_ALLOWED_ORIGINS is wildcard (*) but HTTP_ADDR %q listens on all interfaces; set an explicit origin list before exposing this service", addr)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return ""
	}
	if strings.EqualFold(host, "localhost") {
		return ""
	}
	return fmt.Sprintf("CORS_ALLOWED_ORIGINS is wildcard (*) but http_addr host %q is not loopback; set an explicit origin list before exposing this service", host)
}

