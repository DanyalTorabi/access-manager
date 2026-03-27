package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func TestHealth(t *testing.T) {
	s := &Server{}
	ts := httptest.NewServer(s.Router())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "ok") {
		t.Fatalf("body: %s", body)
	}
}

// newTestAPI returns an HTTP test server backed by a real SQLite store and migrations.
func newTestAPI(t *testing.T) (*httptest.Server, store.Store) {
	t.Helper()
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "api.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	st := sqlstore.New(db)
	srv := &Server{Store: st}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(func() {
		ts.Close()
		_ = db.Close()
	})
	return ts, st
}

// mustPostJSON201 POSTs JSON and returns the body after asserting http.StatusCreated.
func mustPostJSON201(t *testing.T, urlStr, body string) []byte {
	t.Helper()
	res, err := http.Post(urlStr, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST %s want 201 got %d: %s", urlStr, res.StatusCode, b)
	}
	return b
}

func TestAPI_domainCreateAndList(t *testing.T) {
	ts, _ := newTestAPI(t)

	payload := `{"title":"Acme"}`
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	var created store.Domain
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Title != "Acme" || created.ID == "" {
		t.Fatalf("domain: %+v", created)
	}

	res2, err := http.Get(ts.URL + "/api/v1/domains")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", res2.StatusCode)
	}
	var list []store.Domain
	if err := json.NewDecoder(res2.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list: %+v", list)
	}
}

func TestAPI_userGet_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := uuid.NewString()
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/users/" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res.StatusCode)
	}
}

func TestAPI_authzCheck_validation(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := uuid.NewString()
	url := ts.URL + "/api/v1/domains/" + domainID + "/authz/check?user_id=u&resource_id=r"
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 missing access_bit, got %d", res.StatusCode)
	}
}

func TestAPI_authzCheck_viaGroup_integration(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()

	domainID := uuid.NewString()
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()

	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := st.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x3}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := st.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}

	q := ts.URL + "/api/v1/domains/" + domainID + "/authz/check?user_id=" + uid + "&resource_id=" + rid + "&access_bit=0x1"
	res, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	var out struct {
		Allowed       bool   `json:"allowed"`
		EffectiveMask string `json:"effective_mask"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Allowed {
		t.Fatalf("expected allowed, got %+v", out)
	}
}

func TestAPI_domainCreate_invalidJSON(t *testing.T) {
	ts, _ := newTestAPI(t)
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(`{"title":`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, body)
	}
}

func TestAPI_domainCreate_unknownField(t *testing.T) {
	ts, _ := newTestAPI(t)
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(`{"title":"x","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400 unknown field, got %d: %s", res.StatusCode, body)
	}
}

func TestAPI_permissionCreate_invalidMask(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	if dom.ID == "" {
		t.Fatal("empty domain id")
	}

	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	if resource.ID == "" {
		t.Fatal("empty resource id")
	}

	body := fmt.Sprintf(`{"title":"p","resource_id":"%s","access_mask":"not-a-number"}`, resource.ID)
	res3, err := http.Post(ts.URL+"/api/v1/domains/"+dom.ID+"/permissions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res3.Body.Close() }()
	if res3.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res3.Body)
		t.Fatalf("want 400 invalid mask, got %d: %s", res3.StatusCode, b)
	}
}

func TestAPI_authzCheck_invalidAccessBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	did := uuid.NewString()
	url := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=u&resource_id=r&access_bit=xyz", ts.URL, did)
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400 invalid access_bit, got %d: %s", res.StatusCode, body)
	}
}

func TestAPI_authzCheck_deniedWithoutGrants(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	if dom.ID == "" {
		t.Fatal("empty domain id")
	}

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	if user.ID == "" {
		t.Fatal("empty user id")
	}

	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains/"+dom.ID+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	if resource.ID == "" {
		t.Fatal("empty resource id")
	}

	q := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=%s&resource_id=%s&access_bit=0x1",
		ts.URL, dom.ID, user.ID, resource.ID)
	res4, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res4.Body.Close() }()
	if res4.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res4.Body)
		t.Fatalf("status %d: %s", res4.StatusCode, body)
	}
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(res4.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Allowed {
		t.Fatalf("expected denied without grants, got %+v", out)
	}
}

func TestAPI_authzMasks_validation(t *testing.T) {
	ts, _ := newTestAPI(t)
	did := uuid.NewString()
	res, err := http.Get(ts.URL + "/api/v1/domains/" + did + "/authz/masks?user_id=u")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 missing resource_id, got %d", res.StatusCode)
	}
}

func TestAPI_userList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	if dom.ID == "" {
		t.Fatal("empty domain id")
	}

	res2, err := http.Get(ts.URL + "/api/v1/domains/" + dom.ID + "/users")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", res2.StatusCode)
	}
	var list []store.User
	if err := json.NewDecoder(res2.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %+v", list)
	}
}

func TestAPI_health_publicWhenAPIUsesBearer(t *testing.T) {
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "api.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	st := sqlstore.New(db)
	srv := &Server{Store: st, APIBearerToken: "secret-token"}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health should stay public, got %d", res.StatusCode)
	}
}
