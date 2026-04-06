package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mustGet performs a GET and asserts the expected status code.
func mustGet(t *testing.T, url string, wantStatus int) []byte {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("GET %s: want %d, got %d: %s", url, wantStatus, res.StatusCode, b)
	}
	return b
}

// mustDoRequest performs an HTTP request with optional JSON body and asserts status.
func mustDoRequest(t *testing.T, method, url, body string, wantStatus int) []byte {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("%s %s: want %d, got %d: %s", method, url, wantStatus, res.StatusCode, b)
	}
	return b
}

func mustPatchJSON(t *testing.T, url, jsonBody string, wantStatus int) []byte {
	t.Helper()
	return mustDoRequest(t, http.MethodPatch, url, jsonBody, wantStatus)
}

func mustDeleteReq(t *testing.T, url string, wantStatus int) []byte {
	t.Helper()
	return mustDoRequest(t, http.MethodDelete, url, "", wantStatus)
}

func mustPostJSON(t *testing.T, url, jsonBody string, wantStatus int) []byte {
	t.Helper()
	return mustDoRequest(t, http.MethodPost, url, jsonBody, wantStatus)
}

func mustPostEmpty(t *testing.T, url string, wantStatus int) []byte {
	t.Helper()
	return mustDoRequest(t, http.MethodPost, url, "", wantStatus)
}

func domainBase(ts *httptest.Server, domainID string) string {
	return ts.URL + "/api/v1/domains/" + domainID
}

func seedDomain(t *testing.T, ts *httptest.Server, title string) string {
	t.Helper()
	b := mustPostJSON(t, ts.URL+"/api/v1/domains", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func seedUser(t *testing.T, ts *httptest.Server, domainID, title string) string {
	t.Helper()
	b := mustPostJSON(t, domainBase(ts, domainID)+"/users", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func seedGroup(t *testing.T, ts *httptest.Server, domainID, title string) string {
	t.Helper()
	b := mustPostJSON(t, domainBase(ts, domainID)+"/groups", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func seedResource(t *testing.T, ts *httptest.Server, domainID, title string) string {
	t.Helper()
	b := mustPostJSON(t, domainBase(ts, domainID)+"/resources", fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func seedAccessType(t *testing.T, ts *httptest.Server, domainID, title, bit string) string {
	t.Helper()
	b := mustPostJSON(t, domainBase(ts, domainID)+"/access-types",
		fmt.Sprintf(`{"title":%q,"bit":%q}`, title, bit), http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func seedPermission(t *testing.T, ts *httptest.Server, domainID, title, resourceID, mask string) string {
	t.Helper()
	b := mustPostJSON(t, domainBase(ts, domainID)+"/permissions",
		fmt.Sprintf(`{"title":%q,"resource_id":%q,"access_mask":%q}`, title, resourceID, mask), http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

func addMembership(t *testing.T, ts *httptest.Server, domainID, userID, groupID string) {
	t.Helper()
	mustPostEmpty(t, domainBase(ts, domainID)+"/users/"+userID+"/groups/"+groupID, http.StatusNoContent)
}

func grantUserPerm(t *testing.T, ts *httptest.Server, domainID, userID, permID string) {
	t.Helper()
	mustPostEmpty(t, domainBase(ts, domainID)+"/users/"+userID+"/permissions/"+permID, http.StatusNoContent)
}

func grantGroupPerm(t *testing.T, ts *httptest.Server, domainID, groupID, permID string) {
	t.Helper()
	mustPostEmpty(t, domainBase(ts, domainID)+"/groups/"+groupID+"/permissions/"+permID, http.StatusNoContent)
}

func revokeUserPerm(t *testing.T, ts *httptest.Server, domainID, userID, permID string) {
	t.Helper()
	mustDeleteReq(t, domainBase(ts, domainID)+"/users/"+userID+"/permissions/"+permID, http.StatusNoContent)
}

// doRequestErr is a goroutine-safe variant of mustDoRequest that returns the
// response body and an error instead of calling t.Fatal (which is unsafe from
// non-test goroutines).
func doRequestErr(method, url, body string, wantStatus int) ([]byte, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%s %s: build request: %w", method, url, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: read body: %w", method, url, err)
	}
	if res.StatusCode != wantStatus {
		return nil, fmt.Errorf("%s %s: want %d, got %d: %s", method, url, wantStatus, res.StatusCode, b)
	}
	return b, nil
}
