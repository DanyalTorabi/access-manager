//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Bearer auth journey — only meaningful when API_BEARER_TOKEN is set.
// ---------------------------------------------------------------------------

func TestAuth_bearerJourney(t *testing.T) {
	tok := strings.TrimSpace(os.Getenv("API_BEARER_TOKEN"))
	if tok == "" {
		t.Skip("API_BEARER_TOKEN not set; skipping auth journey")
	}

	c := httpClient()

	t.Run("missing_token", func(t *testing.T) {
		doRaw(t, c, http.MethodGet, apiBase()+"/domains", "", http.StatusUnauthorized)
	})

	t.Run("wrong_token", func(t *testing.T) {
		doRaw(t, c, http.MethodGet, apiBase()+"/domains", "wrong-token-value", http.StatusUnauthorized)
	})

	t.Run("valid_token", func(t *testing.T) {
		doRaw(t, c, http.MethodGet, apiBase()+"/domains", tok, http.StatusOK)
	})

	t.Run("health_no_auth", func(t *testing.T) {
		doRaw(t, c, http.MethodGet, baseURL()+"/health", "", http.StatusOK)
	})
}
