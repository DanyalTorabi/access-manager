package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

func dummyRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/test", nil)
}

func TestWriteStoreErr_allCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		err     error
		want    int
		wantMsg string
	}{
		{"not found", store.ErrNotFound, http.StatusNotFound, "resource not found"},
		{"fk violation", store.ErrFKViolation, http.StatusBadRequest, "referenced entity does not exist or is still referenced"},
		{"invalid input", store.ErrInvalidInput, http.StatusBadRequest, "invalid request"},
		{"invalid input detail", store.NewInvalidInput("cycle detected in group parent chain"), http.StatusBadRequest, "cycle detected in group parent chain"},
		{"invalid input wrapped detail", fmt.Errorf("ctx: %w", store.NewInvalidInput("empty patch")), http.StatusBadRequest, "empty patch"},
		{"invalid input mask range", store.NewInvalidInput(store.InvalidInputDetailMaskOverflow), http.StatusBadRequest, "mask value must be within signed 64-bit range"},
		{"conflict", store.ErrConflict, http.StatusConflict, "resource already exists"},
		{"generic", fmt.Errorf("boom"), http.StatusInternalServerError, "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			s := &Server{Log: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))}
			w := httptest.NewRecorder()
			s.writeStoreErr(w, dummyRequest(), tt.err)
			if w.Code != tt.want {
				t.Fatalf("writeStoreErr(%v) = %d, want %d", tt.err, w.Code, tt.want)
			}
			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tt.wantMsg {
				t.Fatalf("body error = %q, want %q", body["error"], tt.wantMsg)
			}
			if !strings.Contains(buf.String(), tt.err.Error()) {
				t.Fatal("full error not logged")
			}
		})
	}
}

func TestWriteStoreErr_noSQLLeak(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := &Server{Log: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))}

	sqlDetail := "FOREIGN KEY constraint failed (errno 787)"
	joined := fmt.Errorf("%w\n%s", store.ErrFKViolation, sqlDetail)

	w := httptest.NewRecorder()
	s.writeStoreErr(w, dummyRequest(), joined)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	respBody := w.Body.String()
	for _, leak := range []string{"FOREIGN KEY", "constraint", "errno", "sqlite"} {
		if strings.Contains(strings.ToLower(respBody), strings.ToLower(leak)) {
			t.Fatalf("response body leaked %q: %s", leak, respBody)
		}
	}
	if !strings.Contains(buf.String(), sqlDetail) {
		t.Fatal("full SQL error not logged server-side")
	}
}

func TestWriteInternalErr_generic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := &Server{Log: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))}

	w := httptest.NewRecorder()
	s.writeInternalErr(w, dummyRequest(), fmt.Errorf("sql: database is closed"))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	respBytes := w.Body.Bytes()
	var body map[string]string
	if err := json.Unmarshal(respBytes, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Fatalf("body = %q, want generic", body["error"])
	}
	if strings.Contains(string(respBytes), "database is closed") {
		t.Fatal("raw error leaked to client")
	}
	if !strings.Contains(buf.String(), "database is closed") {
		t.Fatal("full error not logged")
	}
}

// TestWriteInternalErr_misuse asserts that passing a structured store sentinel
// (e.g. ErrNotFound) to writeInternalErr logs an ERROR-level misuse alert while
// still returning a generic 500 to the client — making the wrong call site
// immediately visible in production.
func TestWriteInternalErr_misuse(t *testing.T) {
	t.Parallel()
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", store.ErrNotFound},
		{"ErrConflict", store.ErrConflict},
		{"ErrFKViolation", store.ErrFKViolation},
		{"ErrInvalidInput", store.ErrInvalidInput},
	}
	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			s := &Server{Log: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))}

			w := httptest.NewRecorder()
			s.writeInternalErr(w, dummyRequest(), tt.err)

			if w.Code != http.StatusInternalServerError {
				t.Fatalf("want 500, got %d", w.Code)
			}
			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != "internal server error" {
				t.Fatalf("body[error] = %q, want generic 500", body["error"])
			}
			if !strings.Contains(buf.String(), "writeInternalErr misuse") {
				t.Fatalf("expected misuse alert in log, got: %s", buf.String())
			}
		})
	}
}

// --- store-error tests using a broken (closed-DB) store ---

// newBrokenTestAPIWithRegistry builds a Server backed by a closed DB so any
func TestAPI_storeErrors(t *testing.T) {
	t.Parallel()
	ts := newBrokenTestAPI(t)
	domID := uuid.NewString()
	userID := uuid.NewString()
	groupID := uuid.NewString()
	resourceID := uuid.NewString()
	permID := uuid.NewString()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"domainCreate", http.MethodPost, "/api/v1/domains", `{"title":"d"}`, 500},
		{"domainList", http.MethodGet, "/api/v1/domains", "", 500},
		{"userCreate", http.MethodPost, "/api/v1/domains/" + domID + "/users", `{"title":"u"}`, 500},
		{"userList", http.MethodGet, "/api/v1/domains/" + domID + "/users", "", 500},
		{"userGet", http.MethodGet, "/api/v1/domains/" + domID + "/users/" + userID, "", 500},
		{"groupCreate", http.MethodPost, "/api/v1/domains/" + domID + "/groups", `{"title":"g"}`, 500},
		{"groupList", http.MethodGet, "/api/v1/domains/" + domID + "/groups", "", 500},
		{"groupGet", http.MethodGet, "/api/v1/domains/" + domID + "/groups/" + groupID, "", 500},
		{"resourceCreate", http.MethodPost, "/api/v1/domains/" + domID + "/resources", `{"title":"r"}`, 500},
		{"resourceList", http.MethodGet, "/api/v1/domains/" + domID + "/resources", "", 500},
		{"resourceGet", http.MethodGet, "/api/v1/domains/" + domID + "/resources/" + resourceID, "", 500},
		{"accessTypeCreate", http.MethodPost, "/api/v1/domains/" + domID + "/access-types", `{"title":"read","bit":"0x1"}`, 500},
		{"accessTypeList", http.MethodGet, "/api/v1/domains/" + domID + "/access-types", "", 500},
		{"permissionCreate", http.MethodPost, "/api/v1/domains/" + domID + "/permissions", `{"title":"p","resource_id":"` + resourceID + `","access_mask":"0x1"}`, 500},
		{"permissionList", http.MethodGet, "/api/v1/domains/" + domID + "/permissions", "", 500},
		{"permissionGet", http.MethodGet, "/api/v1/domains/" + domID + "/permissions/" + permID, "", 500},
		{"addUserToGroup", http.MethodPost, "/api/v1/domains/" + domID + "/users/" + userID + "/groups/" + groupID, "", 500},
		{"grantUserPerm", http.MethodPost, "/api/v1/domains/" + domID + "/users/" + userID + "/permissions/" + permID, "", 500},
		{"grantGroupPerm", http.MethodPost, "/api/v1/domains/" + domID + "/groups/" + groupID + "/permissions/" + permID, "", 500},
		{"groupSetParent", http.MethodPatch, "/api/v1/domains/" + domID + "/groups/" + groupID + "/parent", `{"parent_group_id":"` + uuid.NewString() + `"}`, 500},
		{"removeUserFromGroup", http.MethodDelete, "/api/v1/domains/" + domID + "/users/" + userID + "/groups/" + groupID, "", 500},
		{"revokeUserPerm", http.MethodDelete, "/api/v1/domains/" + domID + "/users/" + userID + "/permissions/" + permID, "", 500},
		{"userAuthzResources", http.MethodGet, "/api/v1/domains/" + domID + "/users/" + userID + "/authz/resources", "", 500},
		{"groupAuthzResources", http.MethodGet, "/api/v1/domains/" + domID + "/groups/" + groupID + "/authz/resources", "", 500},
		{"resourceAuthzUsers", http.MethodGet, "/api/v1/domains/" + domID + "/resources/" + resourceID + "/authz/users", "", 500},
		{"resourceAuthzGroups", http.MethodGet, "/api/v1/domains/" + domID + "/resources/" + resourceID + "/authz/groups", "", 500},
		{"revokeGroupPerm", http.MethodDelete, "/api/v1/domains/" + domID + "/groups/" + groupID + "/permissions/" + permID, "", 500},
		{"authzCheck", http.MethodGet, "/api/v1/domains/" + domID + "/authz/check?user_id=" + userID + "&resource_id=" + resourceID + "&access_bit=0x1", "", 500},
		{"authzMasks", http.MethodGet, "/api/v1/domains/" + domID + "/authz/masks?user_id=" + userID + "&resource_id=" + resourceID, "", 500},
		{"domainGet", http.MethodGet, "/api/v1/domains/" + domID, "", 500},
		{"domainPatch", http.MethodPatch, "/api/v1/domains/" + domID, `{"title":"x"}`, 500},
		{"domainDelete", http.MethodDelete, "/api/v1/domains/" + domID, "", 500},
		{"userPatch", http.MethodPatch, "/api/v1/domains/" + domID + "/users/" + userID, `{"title":"x"}`, 500},
		{"userDelete", http.MethodDelete, "/api/v1/domains/" + domID + "/users/" + userID, "", 500},
		{"groupPatch", http.MethodPatch, "/api/v1/domains/" + domID + "/groups/" + groupID, `{"title":"x"}`, 500},
		{"groupDelete", http.MethodDelete, "/api/v1/domains/" + domID + "/groups/" + groupID, "", 500},
		{"resourcePatch", http.MethodPatch, "/api/v1/domains/" + domID + "/resources/" + resourceID, `{"title":"x"}`, 500},
		{"resourceDelete", http.MethodDelete, "/api/v1/domains/" + domID + "/resources/" + resourceID, "", 500},
		{"accessTypeGet", http.MethodGet, "/api/v1/domains/" + domID + "/access-types/" + uuid.NewString(), "", 500},
		{"accessTypePatch", http.MethodPatch, "/api/v1/domains/" + domID + "/access-types/" + uuid.NewString(), `{"title":"x"}`, 500},
		{"accessTypeDelete", http.MethodDelete, "/api/v1/domains/" + domID + "/access-types/" + uuid.NewString(), "", 500},
		{"permissionPatch", http.MethodPatch, "/api/v1/domains/" + domID + "/permissions/" + permID, `{"title":"x"}`, 500},
		{"permissionDelete", http.MethodDelete, "/api/v1/domains/" + domID + "/permissions/" + permID, "", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req, err := http.NewRequest(tt.method, ts.URL+tt.path, body)
			if err != nil {
				t.Fatal(err)
			}
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			if res.StatusCode != tt.want {
				t.Fatalf("want %d, got %d", tt.want, res.StatusCode)
			}
		})
	}
}

// --- pagination tests ---

func TestAPI_patchEmptyBody_allEntities(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + dom

	rBody := mustPostJSON201(t, base+"/resources", `{"title":"r"}`)
	var resrc store.Resource
	if err := json.Unmarshal(rBody, &resrc); err != nil {
		t.Fatal(err)
	}

	atBody := mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)
	var at store.AccessType
	if err := json.Unmarshal(atBody, &at); err != nil {
		t.Fatal(err)
	}

	pBody := mustPostJSON201(t, base+"/permissions", `{"title":"p","resource_id":"`+resrc.ID+`","access_mask":"0x1"}`)
	var perm store.Permission
	if err := json.Unmarshal(pBody, &perm); err != nil {
		t.Fatal(err)
	}

	gBody := mustPostJSON201(t, base+"/groups", `{"title":"g"}`)
	var grp store.Group
	if err := json.Unmarshal(gBody, &grp); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"domain", ts.URL + "/api/v1/domains/" + dom},
		{"resource", base + "/resources/" + resrc.ID},
		{"accessType", base + "/access-types/" + at.ID},
		{"permission", base + "/permissions/" + perm.ID},
		{"group", base + "/groups/" + grp.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPatch, tt.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", res.StatusCode)
			}
		})
	}
}

func TestAPI_pagination_invalidOffset(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	tests := []struct {
		name string
		qs   string
	}{
		{"non-integer offset", "?offset=abc"},
		{"negative offset", "?offset=-1"},
		{"non-integer limit", "?limit=xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Get(ts.URL + "/api/v1/domains" + tt.qs)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", res.StatusCode)
			}
		})
	}
}

func TestAPI_pagination_limitClamping(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains?limit=999")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Limit != 100 {
		t.Fatalf("limit should be clamped to 100, got %d", env.Meta.Limit)
	}
}

func TestAPI_pagination_offsetPastEnd(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains?offset=100")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 {
		t.Fatalf("total: want 1, got %d", env.Meta.Total)
	}
	if len(env.Data) != 0 {
		t.Fatalf("data: want empty, got %d", len(env.Data))
	}
}

func TestAPI_pagination_invalidOffset_otherEndpoints(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + dom

	endpoints := []struct {
		name string
		path string
	}{
		{"users", base + "/users"},
		{"groups", base + "/groups"},
		{"resources", base + "/resources"},
		{"accessTypes", base + "/access-types"},
		{"permissions", base + "/permissions"},
	}
	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			res, err := http.Get(ep.path + "?offset=abc")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", res.StatusCode)
			}
		})
	}
}

func TestAPI_scopedList_pagination(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + dom

	for i := 0; i < 5; i++ {
		title := fmt.Sprintf("u-%c", 'a'+i)
		mustPostJSON201(t, base+"/users", fmt.Sprintf(`{"title":%q}`, title))
	}
	for i := 0; i < 3; i++ {
		title := fmt.Sprintf("g-%c", 'a'+i)
		mustPostJSON201(t, base+"/groups", fmt.Sprintf(`{"title":%q}`, title))
	}
	for i := 0; i < 4; i++ {
		title := fmt.Sprintf("r-%c", 'a'+i)
		mustPostJSON201(t, base+"/resources", fmt.Sprintf(`{"title":%q}`, title))
	}

	tests := []struct {
		name      string
		path      string
		wantTotal int64
	}{
		{"users", base + "/users?offset=1&limit=2", 5},
		{"groups", base + "/groups?offset=0&limit=2", 3},
		{"resources", base + "/resources?offset=2&limit=2", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Get(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = res.Body.Close() }()
			if res.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("status %d: %s", res.StatusCode, b)
			}
			var env struct {
				Meta struct {
					Total  int64 `json:"total"`
					Offset int   `json:"offset"`
					Limit  int   `json:"limit"`
				} `json:"meta"`
			}
			if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
				t.Fatal(err)
			}
			if env.Meta.Total != tt.wantTotal {
				t.Fatalf("total: want %d, got %d", tt.wantTotal, env.Meta.Total)
			}
			if env.Meta.Limit != 2 {
				t.Fatalf("limit: want 2, got %d", env.Meta.Limit)
			}
		})
	}
}

func TestAPI_parseListOpts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		qs             string
		wantOffset     int
		wantLimit      int
		wantSearch     string
		wantSearchType store.SearchType
		wantErr        bool
	}{
		{"defaults", "", 0, 20, "", store.SearchContains, false},
		{"explicit", "offset=5&limit=10", 5, 10, "", store.SearchContains, false},
		{"limit clamped low", "limit=0", 0, 1, "", store.SearchContains, false},
		{"limit clamped high", "limit=200", 0, 100, "", store.SearchContains, false},
		{"bad offset", "offset=abc", 0, 0, "", "", true},
		{"negative offset", "offset=-1", 0, 0, "", "", true},
		{"bad limit", "limit=xyz", 0, 0, "", "", true},
		{"search param", "search=hello", 0, 20, "hello", store.SearchContains, false},
		{"search trimmed", "search=%20hi%20", 0, 20, "hi", store.SearchContains, false},
		{"search with pagination", "search=foo&offset=2&limit=5", 2, 5, "foo", store.SearchContains, false},
		{"search at max length", "search=" + strings.Repeat("a", 255), 0, 20, strings.Repeat("a", 255), store.SearchContains, false},
		{"search too long", "search=" + strings.Repeat("a", 256), 0, 0, "", "", true},
		{"search_type ignored without search", "search_type=starts_with", 0, 20, "", store.SearchContains, false},
		{"search_type invalid ignored without search", "search_type=regex", 0, 20, "", store.SearchContains, false},
		{"search with type contains", "search=foo&search_type=contains", 0, 20, "foo", store.SearchContains, false},
		{"search with type starts_with", "search=foo&search_type=starts_with", 0, 20, "foo", store.SearchStartsWith, false},
		{"search with type ends_with", "search=foo&search_type=ends_with", 0, 20, "foo", store.SearchEndsWith, false},
		{"search with type invalid", "search=foo&search_type=regex", 0, 0, "", "", true},
		{"search with type trimmed", "search=foo&search_type=%20ends_with%20", 0, 20, "foo", store.SearchEndsWith, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test?"+tt.qs, nil)
			opts, err := parseListOpts(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Offset != tt.wantOffset || opts.Limit != tt.wantLimit {
				t.Fatalf("offset=%d limit=%d, want %d/%d", opts.Offset, opts.Limit, tt.wantOffset, tt.wantLimit)
			}
			if opts.Search != tt.wantSearch {
				t.Fatalf("search=%q, want %q", opts.Search, tt.wantSearch)
			}
			if opts.SearchType != tt.wantSearchType {
				t.Fatalf("search_type=%q, want %q", opts.SearchType, tt.wantSearchType)
			}
		})
	}
}

func TestAPI_parseSortOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		qs        string
		allowed   []string
		wantSort  string
		wantOrder store.SortOrder
		wantErr   bool
	}{
		{"defaults", "", store.DomainSortFields, "title", store.OrderAsc, false},
		{"explicit asc", "sort=title&order=asc", store.DomainSortFields, "title", store.OrderAsc, false},
		{"explicit desc", "sort=title&order=desc", store.DomainSortFields, "title", store.OrderDesc, false},
		{"order only", "order=desc", store.DomainSortFields, "title", store.OrderDesc, false},
		{"sort only", "sort=title", store.DomainSortFields, "title", store.OrderAsc, false},
		{"permission resource_id", "sort=resource_id", store.PermissionSortFields, "resource_id", store.OrderAsc, false},
		{"invalid sort", "sort=unknown", store.DomainSortFields, "", "", true},
		{"invalid order", "order=random", store.DomainSortFields, "", "", true},
		{"sort trimmed", "sort=%20title%20", store.DomainSortFields, "title", store.OrderAsc, false},
		{"order trimmed", "order=%20desc%20", store.DomainSortFields, "title", store.OrderDesc, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test?"+tt.qs, nil)
			sort, order, err := parseSortOrder(req.URL.Query(), tt.allowed)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sort != tt.wantSort {
				t.Fatalf("sort=%q, want %q", sort, tt.wantSort)
			}
			if order != tt.wantOrder {
				t.Fatalf("order=%q, want %q", order, tt.wantOrder)
			}
		})
	}
}

func TestAPI_readJSON_tooLargeBody(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	bigBody := `{"title":"` + strings.Repeat("x", 2*1024*1024) + `"}`
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(bigBody))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", res.StatusCode)
	}
}

// TestPublicInvalidInputMsg_typedExtraction asserts that publicInvalidInputMsg
// uses errors.As (not string-prefix parsing): a typed
// store.InvalidInputError detail must be returned to the client even when
// wrapped by an outer fmt.Errorf("%w", err). This is the regression that T48
// fixed.
func TestPublicInvalidInputMsg_typedExtraction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"plain_typed", store.NewInvalidInput("empty patch"), "empty patch"},
		{"wrapped_once", fmt.Errorf("ctx: %w", store.NewInvalidInput("empty patch")), "empty patch"},
		{"wrapped_twice", fmt.Errorf("a: %w", fmt.Errorf("b: %w", store.NewInvalidInput("cycle detected in group parent chain"))), "cycle detected in group parent chain"},
		{"mask_overflow_translated", store.NewInvalidInput(store.InvalidInputDetailMaskOverflow), "mask value must be within signed 64-bit range"},
		{"plain_sentinel_falls_back", store.ErrInvalidInput, "invalid request"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := publicInvalidInputMsg(c.err); got != c.want {
				t.Fatalf("publicInvalidInputMsg = %q, want %q", got, c.want)
			}
		})
	}
}

// --- T47: parseUint64Validated and stable numeric parse errors ---

// TestParseUint64Validated covers the validated parse helper directly: every
// rejection path returns a stable sentinel error and never embeds the raw
// strconv message or the user input.
func TestParseUint64Validated(t *testing.T) {
	t.Parallel()
	const max = maxAccessMask // 1<<63 - 1
	type tc struct {
		name    string
		in      string
		max     uint64
		want    uint64
		wantErr error
	}
	cases := []tc{
		{"decimal_zero", "0", max, 0, nil},
		{"decimal_small", "42", max, 42, nil},
		{"hex_lower", "0x1f", max, 31, nil},
		{"hex_upper", "0X10", max, 16, nil},
		{"max_signed64_ok", "0x7FFFFFFFFFFFFFFF", max, 1<<63 - 1, nil},
		{"max_disabled_accepts_full_uint64", "0xFFFFFFFFFFFFFFFF", 0, ^uint64(0), nil},
		// Leading-zero decimals are not interpreted as octal (which strconv
		// base 0 would do — "010" -> 8). Helper uses base 10 so "010" -> 10.
		{"leading_zero_decimal", "010", max, 10, nil},

		{"empty_string", "", max, 0, errInvalidNumericValue},
		{"non_numeric", "notanumber", max, 0, errInvalidNumericValue},
		{"trailing_garbage", "12abc", max, 0, errInvalidNumericValue},
		{"negative", "-1", max, 0, errInvalidNumericValue},
		{"plus_sign", "+1", max, 0, errInvalidNumericValue},
		{"malformed_hex", "0xZZ", max, 0, errInvalidNumericValue},
		{"overflow_uint64", "0x10000000000000000", max, 0, errInvalidNumericValue},
		// strconv.ParseUint(base=0) would accept these; the helper must
		// reject them so the wire format stays "decimal or 0x hex" as
		// documented in api/openapi.yaml.
		{"binary_rejected", "0b10", max, 0, errInvalidNumericValue},
		{"hex_prefix_only", "0x", max, 0, errInvalidNumericValue},
		{"leading_whitespace", " 1", max, 0, errInvalidNumericValue},
		{"trailing_whitespace", "1 ", max, 0, errInvalidNumericValue},
		// Defensive length cap (maxNumericInputLen).
		{"too_long", strings.Repeat("9", 33), max, 0, errInvalidNumericValue},

		{"out_of_range_bit63", "0x8000000000000000", max, 0, errAccessMaskOutOfRange},
		{"out_of_range_max_uint64", "0xFFFFFFFFFFFFFFFF", max, 0, errAccessMaskOutOfRange},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseUint64Validated(c.in, c.max)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if err == nil && got != c.want {
				t.Fatalf("got = %d, want %d", got, c.want)
			}
			if err != nil {
				if strings.Contains(err.Error(), c.in) && c.in != "" {
					t.Fatalf("error message %q must not echo input %q", err.Error(), c.in)
				}
				if strings.Contains(err.Error(), "strconv") {
					t.Fatalf("error message %q leaks strconv internals", err.Error())
				}
			}
		})
	}
}

// TestAPI_numericParseErrors_stableMessages asserts that the API surfaces the
// stable "invalid numeric value" message (not strconv text) for malformed
// numeric input on every handler that parses bit / access_mask. Status code
// is asserted before the body is read.
func TestAPI_numericParseErrors_stableMessages(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + dom
	rBody := mustPostJSON201(t, base+"/resources", `{"title":"r"}`)
	var resrc store.Resource
	if err := json.Unmarshal(rBody, &resrc); err != nil {
		t.Fatal(err)
	}
	atBody := mustPostJSON201(t, base+"/access-types", `{"title":"a","bit":"0x1"}`)
	var at store.AccessType
	if err := json.Unmarshal(atBody, &at); err != nil {
		t.Fatal(err)
	}
	pBody := mustPostJSON201(t, base+"/permissions", fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resrc.ID))
	var perm store.Permission
	if err := json.Unmarshal(pBody, &perm); err != nil {
		t.Fatal(err)
	}

	const wantMsg = "invalid numeric value"
	const badInput = "notanumber"
	assertBad := func(t *testing.T, res *http.Response) {
		t.Helper()
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode != http.StatusBadRequest {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
		}
		var body map[string]string
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		got := body["error"]
		if got != wantMsg {
			t.Fatalf(`body["error"] = %q, want %q`, got, wantMsg)
		}
		if strings.Contains(got, badInput) {
			t.Fatalf(`body["error"] echoes input: %q`, got)
		}
	}
	doPatch := func(t *testing.T, url, body string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	t.Run("accessTypeCreate", func(t *testing.T) {
		res, err := http.Post(base+"/access-types", "application/json",
			strings.NewReader(`{"title":"x","bit":"`+badInput+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		assertBad(t, res)
	})
	t.Run("accessTypePatch", func(t *testing.T) {
		assertBad(t, doPatch(t, base+"/access-types/"+at.ID, `{"bit":"`+badInput+`"}`))
	})
	t.Run("permissionCreate", func(t *testing.T) {
		res, err := http.Post(base+"/permissions", "application/json",
			strings.NewReader(fmt.Sprintf(`{"title":"p2","resource_id":%q,"access_mask":"`+badInput+`"}`, resrc.ID)))
		if err != nil {
			t.Fatal(err)
		}
		assertBad(t, res)
	})
	t.Run("permissionPatch", func(t *testing.T) {
		assertBad(t, doPatch(t, base+"/permissions/"+perm.ID, `{"access_mask":"`+badInput+`"}`))
	})
	t.Run("authzCheck_accessBit", func(t *testing.T) {
		url := fmt.Sprintf("%s/authz/check?user_id=u&resource_id=r&access_bit=%s", base, badInput)
		res, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		assertBad(t, res)
	})
}

// TestWriteJSON_encodeErrorLogged asserts that a response encoding failure is
// logged at ERROR level with method and path so operators can identify the
// failing endpoint. Because the status header is already committed, the only
// observable signal is the log entry.
func TestWriteJSON_encodeErrorLogged(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	s := &Server{Log: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	// A channel is not JSON-serializable and will cause Encode to fail.
	s.writeJSON(w, r, http.StatusOK, map[string]any{"v": make(chan int)})

	if w.Code != http.StatusOK {
		t.Fatalf("want status 200 (header already committed), got %d", w.Code)
	}

	logged := strings.TrimSpace(buf.String())
	if logged == "" {
		t.Fatal("expected encode-failure log entry, got empty log output")
	}

	lines := strings.Split(logged, "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v; raw log: %s", err, logged)
	}

	if got := entry["msg"]; got != "response encode failed" {
		t.Fatalf(`log field "msg" = %v, want %q; raw log: %s`, got, "response encode failed", logged)
	}
	if got := entry["method"]; got != http.MethodGet {
		t.Fatalf(`log field "method" = %v, want %q; raw log: %s`, got, http.MethodGet, logged)
	}
	if got := entry["path"]; got != "/api/v1/domains" {
		t.Fatalf(`log field "path" = %v, want %q; raw log: %s`, got, "/api/v1/domains", logged)
	}
}

// TestReadJSON_trailingDataRejected asserts that sending two JSON objects in a
// single request body returns 400 with a stable, client-safe message.
func TestReadJSON_trailingDataRejected(t *testing.T) {
	t.Parallel()
	ts, _ := newTestAPI(t)
	// Second JSON object after the first — trailing data.
	body := `{"title":"first"}{"title":"second"}`
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400 for trailing JSON, got %d: %s", res.StatusCode, b)
	}
	var out map[string]string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	const wantMsg = "request body must contain exactly one JSON value"
	if got := out["error"]; got != wantMsg {
		t.Fatalf(`body["error"] = %q, want %q`, got, wantMsg)
	}
}

// TestReadJSON_failureKindsLogged asserts that readJSON logs the correct
// structured kind label for each class of decode failure.
func TestReadJSON_failureKindsLogged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		wantKind   string
		wantStatus int
	}{
		// Empty body → io.EOF → kind=empty_body.
		{"empty_body", ``, "empty_body", http.StatusBadRequest},
		// Truncated body → io.ErrUnexpectedEOF → kind=json_syntax.
		{"syntax_error", `{"title":`, "json_syntax", http.StatusBadRequest},
		// Wrong JSON type for a string field → *json.UnmarshalTypeError → kind=json_type.
		{"type_error", `{"title":123}`, "json_type", http.StatusBadRequest},
		// Unknown field → kind=json_unknown_field.
		{"unknown_field", `{"title":"x","injected":1}`, "json_unknown_field", http.StatusBadRequest},
		// Trailing data after first value → kind=trailing_data.
		{"trailing_data", `{"title":"x"}{"extra":1}`, "trailing_data", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts, _, logBuf := newTestAPIWithLog(t)
			res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("want %d, got %d", tt.wantStatus, res.StatusCode)
			}

			// Scan all log entries first, then assert. Failing on the first
			// matching entry would mask later entries with the correct kind.
			var foundKind string
			for _, line := range strings.Split(logBuf.String(), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					continue
				}
				if entry["msg"] != "request body decode failed" {
					continue
				}
				foundKind, _ = entry["kind"].(string)
				break // take the first matching log entry
			}
			if foundKind == "" {
				t.Fatalf("no 'request body decode failed' log entry found\nlog: %s", logBuf.String())
			}
			if foundKind != tt.wantKind {
				t.Fatalf("kind=%q, want %q\nlog: %s", foundKind, tt.wantKind, logBuf.String())
			}
		})
	}
}

// TestReadJSON_bodyTooLargeKindLogged asserts that a body exceeding the 1 MiB
// limit returns 413 and logs kind=body_too_large.
func TestReadJSON_bodyTooLargeKindLogged(t *testing.T) {
	t.Parallel()
	ts, _, logBuf := newTestAPIWithLog(t)
	// Build a body just over 1 MiB; wrap in a JSON object so it parses until
	// the size limit is hit.
	oversized := `{"title":"` + strings.Repeat("x", maxRequestBodySize+1) + `"}`
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(oversized))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", res.StatusCode)
	}

	var foundKind string
	for _, line := range strings.Split(logBuf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry["msg"] == "request body decode failed" {
			foundKind, _ = entry["kind"].(string)
			break
		}
	}
	if foundKind != "body_too_large" {
		t.Fatalf("want kind=body_too_large in log, got %q\nlog: %s", foundKind, logBuf.String())
	}
}

// TestReadJSON_clientMessagesDoNotLeakRawInput asserts that the client-visible
// error messages for decode failures are stable and never include raw user input
// (e.g. invalid characters, field names from the request body).
func TestReadJSON_clientMessagesDoNotLeakRawInput(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"empty_body", ``, "request body must not be empty"},
		{"syntax_error", `{"title":`, "request body contains malformed JSON"},
		{"type_error", `{"title":123}`, "request body contains an invalid field value"},
		{"unknown_field", `{"title":"x","injected_field":1}`, "invalid request body"},
		{"trailing_data", `{"title":"x"}{"extra":1}`, "request body must contain exactly one JSON value"},
	}

	ts, _ := newTestAPI(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = res.Body.Close() }()
			if res.StatusCode != http.StatusBadRequest {
				b, _ := io.ReadAll(res.Body)
				t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
			}
			var out map[string]string
			if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got := out["error"]; got != tt.wantMsg {
				t.Fatalf(`body["error"] = %q, want %q`, got, tt.wantMsg)
			}
		})
	}
}
