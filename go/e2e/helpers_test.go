//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Config / client
// ---------------------------------------------------------------------------

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

func apiBase() string { return baseURL() + "/api/v1" }

func domainBase(domainID string) string {
	return apiBase() + "/domains/" + domainID
}

// ---------------------------------------------------------------------------
// Generic HTTP verb helpers
// ---------------------------------------------------------------------------

// mustDo performs an HTTP request, asserts the status code, and returns the body.
func mustDo(t *testing.T, c *http.Client, method, urlStr, body string, want int) []byte {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
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
		t.Fatalf("%s %s want status %d got %d body: %s", method, urlStr, want, res.StatusCode, b)
	}
	return b
}

// doRaw performs a request WITHOUT authHeader — useful for auth journey tests.
func doRaw(t *testing.T, c *http.Client, method, urlStr, bearer string, want int) []byte {
	t.Helper()
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
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
		t.Fatalf("%s %s want status %d got %d body: %s", method, urlStr, want, res.StatusCode, b)
	}
	return b
}

func mustGET(t *testing.T, c *http.Client, urlStr string, want int) []byte {
	t.Helper()
	return mustDo(t, c, http.MethodGet, urlStr, "", want)
}

func mustPostJSON(t *testing.T, c *http.Client, urlStr, body string, want int) []byte {
	t.Helper()
	return mustDo(t, c, http.MethodPost, urlStr, body, want)
}

func mustPostNoBody(t *testing.T, c *http.Client, urlStr string, want int) {
	t.Helper()
	mustDo(t, c, http.MethodPost, urlStr, "", want)
}

func mustPATCH(t *testing.T, c *http.Client, urlStr, body string, want int) []byte {
	t.Helper()
	return mustDo(t, c, http.MethodPatch, urlStr, body, want)
}

func mustDELETE(t *testing.T, c *http.Client, urlStr string, want int) []byte {
	t.Helper()
	return mustDo(t, c, http.MethodDelete, urlStr, "", want)
}

// ---------------------------------------------------------------------------
// JSON response types
// ---------------------------------------------------------------------------

type entityID struct {
	ID string `json:"ID"`
}

func mustEntityID(t *testing.T, what string, body []byte) string {
	t.Helper()
	var e entityID
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("%s: json: %v body=%s", what, err, body)
	}
	if e.ID == "" {
		t.Fatalf("%s: empty ID in response: %s", what, body)
	}
	return e.ID
}

type healthOK struct {
	Status string `json:"status"`
}

type authzCheckResp struct {
	Allowed bool `json:"allowed"`
}

type authzMasksResp struct {
	Masks         []uint64 `json:"masks"`
	EffectiveMask uint64   `json:"effective_mask"`
}

type listMeta struct {
	Total  int64  `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Sort   string `json:"sort"`
	Order  string `json:"order"`
}

type listEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta listMeta        `json:"meta"`
}

func mustList(t *testing.T, c *http.Client, urlStr string) listEnvelope {
	t.Helper()
	b := mustGET(t, c, urlStr, http.StatusOK)
	var env listEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("decode list envelope: %v body=%s", err, b)
	}
	return env
}

// entityTitle is the minimal shape shared by most entity GET/PATCH responses.
type entityTitle struct {
	ID    string `json:"ID"`
	Title string `json:"Title"`
}

// ---------------------------------------------------------------------------
// Seed helpers — create entities and return their IDs
// ---------------------------------------------------------------------------

func seedDomain(t *testing.T, c *http.Client, title string) string {
	t.Helper()
	return mustEntityID(t, "create domain",
		mustPostJSON(t, c, apiBase()+"/domains", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated))
}

func seedUser(t *testing.T, c *http.Client, domainID, title string) string {
	t.Helper()
	return mustEntityID(t, "create user",
		mustPostJSON(t, c, domainBase(domainID)+"/users", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated))
}

func seedGroup(t *testing.T, c *http.Client, domainID, title string) string {
	t.Helper()
	return mustEntityID(t, "create group",
		mustPostJSON(t, c, domainBase(domainID)+"/groups", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated))
}

func seedGroupWithParent(t *testing.T, c *http.Client, domainID, title, parentID string) string {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q,"parent_group_id":%q}`, title, parentID)
	return mustEntityID(t, "create child group",
		mustPostJSON(t, c, domainBase(domainID)+"/groups", body, http.StatusCreated))
}

func seedResource(t *testing.T, c *http.Client, domainID, title string) string {
	t.Helper()
	return mustEntityID(t, "create resource",
		mustPostJSON(t, c, domainBase(domainID)+"/resources", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated))
}

func seedAccessType(t *testing.T, c *http.Client, domainID, title, bit string) string {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q,"bit":%q}`, title, bit)
	return mustEntityID(t, "create access type",
		mustPostJSON(t, c, domainBase(domainID)+"/access-types", body, http.StatusCreated))
}

func seedPermission(t *testing.T, c *http.Client, domainID, title, resourceID, mask string) string {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q,"resource_id":%q,"access_mask":%q}`, title, resourceID, mask)
	return mustEntityID(t, "create permission",
		mustPostJSON(t, c, domainBase(domainID)+"/permissions", body, http.StatusCreated))
}

func addMembership(t *testing.T, c *http.Client, domainID, userID, groupID string) {
	t.Helper()
	mustPostNoBody(t, c, domainBase(domainID)+"/users/"+userID+"/groups/"+groupID, http.StatusNoContent)
}

func grantUserPerm(t *testing.T, c *http.Client, domainID, userID, permID string) {
	t.Helper()
	mustPostNoBody(t, c, domainBase(domainID)+"/users/"+userID+"/permissions/"+permID, http.StatusNoContent)
}

func grantGroupPerm(t *testing.T, c *http.Client, domainID, groupID, permID string) {
	t.Helper()
	mustPostNoBody(t, c, domainBase(domainID)+"/groups/"+groupID+"/permissions/"+permID, http.StatusNoContent)
}

func revokeUserPerm(t *testing.T, c *http.Client, domainID, userID, permID string) {
	t.Helper()
	mustDELETE(t, c, domainBase(domainID)+"/users/"+userID+"/permissions/"+permID, http.StatusNoContent)
}

func revokeGroupPerm(t *testing.T, c *http.Client, domainID, groupID, permID string) {
	t.Helper()
	mustDELETE(t, c, domainBase(domainID)+"/groups/"+groupID+"/permissions/"+permID, http.StatusNoContent)
}

func removeMembership(t *testing.T, c *http.Client, domainID, userID, groupID string) {
	t.Helper()
	mustDELETE(t, c, domainBase(domainID)+"/users/"+userID+"/groups/"+groupID, http.StatusNoContent)
}

// assertAuthzCheck verifies /authz/check for the given user, resource, and access bit.
func assertAuthzCheck(t *testing.T, c *http.Client, domainID, userID, resourceID, bit string, wantAllowed bool) {
	t.Helper()
	url := fmt.Sprintf("%s/authz/check?user_id=%s&resource_id=%s&access_bit=%s",
		domainBase(domainID), userID, resourceID, bit)
	var out authzCheckResp
	if err := json.Unmarshal(mustGET(t, c, url, http.StatusOK), &out); err != nil {
		t.Fatal(err)
	}
	if out.Allowed != wantAllowed {
		t.Fatalf("authz/check(user=%s, res=%s, bit=%s): want allowed=%v, got %v",
			userID, resourceID, bit, wantAllowed, out.Allowed)
	}
}
