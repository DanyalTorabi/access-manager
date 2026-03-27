//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func baseURL() string {
	u := strings.TrimSpace(os.Getenv("BASE_URL"))
	if u == "" {
		u = "http://127.0.0.1:8080"
	}
	return strings.TrimSuffix(u, "/")
}

func authHeader(req *http.Request) {
	tok := strings.TrimSpace(os.Getenv("API_BEARER_TOKEN"))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func mustPostJSON(t *testing.T, c *http.Client, urlStr, body string, want int) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	authHeader(req)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("POST %s want status %d got %d body: %s", urlStr, want, res.StatusCode, b)
	}
	return b
}

func mustGET(t *testing.T, c *http.Client, urlStr string, want int) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	authHeader(req)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("GET %s want status %d got %d body: %s", urlStr, want, res.StatusCode, b)
	}
	return b
}

func mustPostNoBody(t *testing.T, c *http.Client, urlStr string, want int) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, urlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	authHeader(req)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("POST %s want status %d got %d body: %s", urlStr, want, res.StatusCode, b)
	}
}

type entityID struct {
	ID string `json:"ID"`
}

type healthOK struct {
	Status string `json:"status"`
}

type authzCheck struct {
	Allowed bool `json:"allowed"`
}

// TestSmoke_fullJourney mirrors test/e2e/bash/run.sh against BASE_URL.
func TestSmoke_fullJourney(t *testing.T) {
	base := baseURL()
	c := httpClient()

	var h healthOK
	if err := json.Unmarshal(mustGET(t, c, base+"/health", http.StatusOK), &h); err != nil {
		t.Fatal(err)
	}
	if h.Status != "ok" {
		t.Fatalf("health: %+v", h)
	}

	var dom entityID
	if err := json.Unmarshal(mustPostJSON(t, c, base+"/api/v1/domains", `{"title":"e2e-domain"}`, http.StatusCreated), &dom); err != nil {
		t.Fatal(err)
	}
	did := dom.ID

	var user, grp, res entityID
	if err := json.Unmarshal(mustPostJSON(t, c, base+"/api/v1/domains/"+did+"/users", `{"title":"e2e-user"}`, http.StatusCreated), &user); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustPostJSON(t, c, base+"/api/v1/domains/"+did+"/groups", `{"title":"e2e-group"}`, http.StatusCreated), &grp); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustPostJSON(t, c, base+"/api/v1/domains/"+did+"/resources", `{"title":"e2e-resource"}`, http.StatusCreated), &res); err != nil {
		t.Fatal(err)
	}

	permBody := `{"title":"e2e-perm","resource_id":"` + res.ID + `","access_mask":"0x3"}`
	var perm entityID
	if err := json.Unmarshal(mustPostJSON(t, c, base+"/api/v1/domains/"+did+"/permissions", permBody, http.StatusCreated), &perm); err != nil {
		t.Fatal(err)
	}

	mustPostNoBody(t, c, base+"/api/v1/domains/"+did+"/users/"+user.ID+"/groups/"+grp.ID, http.StatusNoContent)
	mustPostNoBody(t, c, base+"/api/v1/domains/"+did+"/groups/"+grp.ID+"/permissions/"+perm.ID, http.StatusNoContent)

	checkURL := base + "/api/v1/domains/" + did + "/authz/check?user_id=" + user.ID + "&resource_id=" + res.ID + "&access_bit=0x1"
	var out authzCheck
	if err := json.Unmarshal(mustGET(t, c, checkURL, http.StatusOK), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Allowed {
		t.Fatalf("authz check: %+v", out)
	}
}
