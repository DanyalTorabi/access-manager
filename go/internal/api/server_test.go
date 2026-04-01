package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
	sqlstore "github.com/dtorabi/access-manager/internal/store/sqlite"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func TestHealth(t *testing.T) {
	s := &Server{}
	ts := httptest.NewServer(s.Router(nil, nil))
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

// newTestStore returns a migrated SQLite store and a cleanup function.
func newTestStore(t *testing.T) (store.Store, func()) {
	t.Helper()
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "api.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return sqlstore.New(db), func() { _ = db.Close() }
}

// newTestAPI returns an HTTP test server backed by a real SQLite store and migrations.
func newTestAPI(t *testing.T) (*httptest.Server, store.Store) {
	t.Helper()
	st, cleanup := newTestStore(t)
	srv := &Server{Store: st}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(func() {
		ts.Close()
		cleanup()
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

// auditLogEntries returns each newline-delimited JSON object from buf that has audit=true.
func auditLogEntries(t *testing.T, buf string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, rawLine := range strings.Split(buf, "\n") {
		rawLine = strings.TrimSpace(rawLine)
		if rawLine == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(rawLine), &m); err != nil {
			t.Fatalf("log line JSON: %v — line %q — full buf: %q", err, rawLine, buf)
		}
		if v, ok := m["audit"]; ok && v == true {
			out = append(out, m)
		}
	}
	return out
}

func auditLogEntriesWithAction(t *testing.T, buf, action string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, e := range auditLogEntries(t, buf) {
		if e["action"] == action {
			out = append(out, e)
		}
	}
	return out
}

func TestAPI_auditLog_domainCreate(t *testing.T) {
	var buf bytes.Buffer
	logger.Init(slog.LevelInfo, &buf)
	t.Cleanup(func() { logger.Init(slog.LevelInfo, os.Stderr) })

	ts, _ := newTestAPI(t)
	payload := `{"title":"AuditCo"}`
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var created store.Domain
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	domainAudits := auditLogEntriesWithAction(t, buf.String(), "domain_create")
	if len(domainAudits) != 1 {
		t.Fatalf("want 1 domain_create audit, got %d in %q", len(domainAudits), buf.String())
	}
	line := domainAudits[0]
	if line["msg"] != "audit" {
		t.Fatalf("want msg=audit, got %v", line["msg"])
	}
	if line["domain_id"] != created.ID {
		t.Fatalf("want domain_id=%q, got %v", created.ID, line["domain_id"])
	}
}

func TestAPI_auditLog_groupCreate_parentFields(t *testing.T) {
	var buf bytes.Buffer
	logger.Init(slog.LevelInfo, &buf)
	t.Cleanup(func() { logger.Init(slog.LevelInfo, os.Stderr) })

	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"ad"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	mustPostJSON201(t, base+"/groups", `{"title":"rootg"}`)
	groups := auditLogEntriesWithAction(t, buf.String(), "group_create")
	if len(groups) != 1 {
		t.Fatalf("want 1 group_create audit after first group, got %d: %q", len(groups), buf.String())
	}
	rootLine := groups[0]
	if rootLine["parent_root"] != true {
		t.Fatalf("want parent_root=true for root group, got %v", rootLine["parent_root"])
	}

	var parent store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"par"}`), &parent); err != nil {
		t.Fatal(err)
	}
	childBody := fmt.Sprintf(`{"title":"ch","parent_group_id":%q}`, parent.ID)
	mustPostJSON201(t, base+"/groups", childBody)
	groups = auditLogEntriesWithAction(t, buf.String(), "group_create")
	if len(groups) != 3 {
		t.Fatalf("want 3 group_create audits after domain + 3 groups, got %d: %q", len(groups), buf.String())
	}
	childLine := groups[len(groups)-1]
	if childLine["parent_group_id"] != parent.ID {
		t.Fatalf("want parent_group_id=%q, got %v", parent.ID, childLine["parent_group_id"])
	}
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
	ts := httptest.NewServer(srv.Router(nil, nil))
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

func TestAPI_groupCreateListGet_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	resEmpty, err := http.Get(base + "/groups")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resEmpty.Body.Close() }()
	if resEmpty.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resEmpty.Body)
		t.Fatalf("list status %d: %s", resEmpty.StatusCode, b)
	}
	var empty []store.Group
	if err := json.NewDecoder(resEmpty.Body).Decode(&empty); err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("want empty groups, got %+v", empty)
	}

	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g1"}`), &g); err != nil {
		t.Fatal(err)
	}
	if g.ID == "" || g.Title != "g1" {
		t.Fatalf("group: %+v", g)
	}

	resList, err := http.Get(base + "/groups")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resList.Body.Close() }()
	if resList.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resList.Body)
		t.Fatalf("list status %d: %s", resList.StatusCode, b)
	}
	var list []store.Group
	if err := json.NewDecoder(resList.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != g.ID {
		t.Fatalf("list: %+v", list)
	}

	resGet, err := http.Get(base + "/groups/" + g.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resGet.Body.Close() }()
	if resGet.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resGet.Body)
		t.Fatalf("get status %d: %s", resGet.StatusCode, b)
	}
	var got store.Group
	if err := json.NewDecoder(resGet.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != g.ID || got.Title != "g1" {
		t.Fatalf("got %+v", got)
	}

	res404, err := http.Get(base + "/groups/" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res404.Body.Close() }()
	if res404.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res404.StatusCode)
	}
}

func TestAPI_groupCreate_invalidJSON(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/api/v1/domains/"+dom.ID+"/groups", "application/json", strings.NewReader(`{"title":`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupCreate_unknownField(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/api/v1/domains/"+dom.ID+"/groups", "application/json", strings.NewReader(`{"title":"g","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupSetParent_toParentAndClear(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var parent, child store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"parent"}`), &parent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"child"}`), &child); err != nil {
		t.Fatal(err)
	}

	patchURL := base + "/groups/" + child.ID + "/parent"
	body := fmt.Sprintf(`{"parent_group_id":%q}`, parent.ID)
	req, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("PATCH set parent want 204, got %d: %s", res.StatusCode, b)
	}

	resGet, err := http.Get(base + "/groups/" + child.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resGet.Body.Close() }()
	if resGet.StatusCode != http.StatusOK {
		t.Fatalf("get status %d", resGet.StatusCode)
	}
	var got store.Group
	if err := json.NewDecoder(resGet.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ParentGroupID == nil || *got.ParentGroupID != parent.ID {
		t.Fatalf("parent not set: %+v", got)
	}

	req2, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(`{"parent_group_id":null}`))
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("PATCH clear parent want 204, got %d: %s", res2.StatusCode, b)
	}

	resCleared, err := http.Get(base + "/groups/" + child.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resCleared.Body.Close() }()
	if resCleared.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resCleared.Body)
		t.Fatalf("GET after clear want 200, got %d: %s", resCleared.StatusCode, b)
	}
	var cleared store.Group
	if err := json.NewDecoder(resCleared.Body).Decode(&cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.ParentGroupID != nil {
		t.Fatalf("parent should be nil after clear, got %v", *cleared.ParentGroupID)
	}
}

func TestAPI_groupSetParent_selfParent(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	patchURL := base + "/groups/" + g.ID + "/parent"
	body := fmt.Sprintf(`{"parent_group_id":%q}`, g.ID)
	req, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400 self-parent, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupSetParent_cycle(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var parentG store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"root"}`), &parentG); err != nil {
		t.Fatal(err)
	}
	childBody := fmt.Sprintf(`{"title":"child","parent_group_id":%q}`, parentG.ID)
	var childG store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", childBody), &childG); err != nil {
		t.Fatal(err)
	}

	patchURL := base + "/groups/" + parentG.ID + "/parent"
	body := fmt.Sprintf(`{"parent_group_id":%q}`, childG.ID)
	req, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400 cycle, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupSetParent_invalidJSON(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	patchURL := base + "/groups/" + g.ID + "/parent"
	req, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(`{"parent_group_id":`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupSetParent_unknownGroup(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"parent"}`), &g); err != nil {
		t.Fatal(err)
	}
	patchURL := base + "/groups/" + uuid.NewString() + "/parent"
	body := fmt.Sprintf(`{"parent_group_id":%q}`, g.ID)
	req, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 404 unknown group, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupSetParent_unknownParent(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"child"}`), &g); err != nil {
		t.Fatal(err)
	}
	patchURL := base + "/groups/" + g.ID + "/parent"
	body := fmt.Sprintf(`{"parent_group_id":%q}`, uuid.NewString())
	req, err := http.NewRequest(http.MethodPatch, patchURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 404 unknown parent, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_resourceListGet_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	resEmpty, err := http.Get(base + "/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resEmpty.Body.Close() }()
	if resEmpty.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resEmpty.StatusCode)
	}
	var empty []store.Resource
	if err := json.NewDecoder(resEmpty.Body).Decode(&empty); err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("want empty, got %+v", empty)
	}

	var r store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r1"}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.ID == "" {
		t.Fatal("empty resource id")
	}

	resList, err := http.Get(base + "/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resList.Body.Close() }()
	if resList.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resList.StatusCode)
	}
	var list []store.Resource
	if err := json.NewDecoder(resList.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != r.ID {
		t.Fatalf("list: %+v", list)
	}

	resGet, err := http.Get(base + "/resources/" + r.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resGet.Body.Close() }()
	if resGet.StatusCode != http.StatusOK {
		t.Fatalf("get status %d", resGet.StatusCode)
	}
	var got store.Resource
	if err := json.NewDecoder(resGet.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID || got.Title != "r1" {
		t.Fatalf("got %+v", got)
	}

	res404, err := http.Get(base + "/resources/" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res404.Body.Close() }()
	if res404.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res404.StatusCode)
	}
}

func TestAPI_accessTypeCreateList_invalidBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	resBad, err := http.Post(base+"/access-types", "application/json", strings.NewReader(`{"title":"read","bit":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resBad.Body.Close() }()
	if resBad.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resBad.Body)
		t.Fatalf("want 400 invalid bit, got %d: %s", resBad.StatusCode, b)
	}

	var at store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`), &at); err != nil {
		t.Fatal(err)
	}
	if at.ID == "" || at.Title != "read" || at.Bit != 1 {
		t.Fatalf("access type: %+v", at)
	}

	resList, err := http.Get(base + "/access-types")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resList.Body.Close() }()
	if resList.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resList.StatusCode)
	}
	var list []store.AccessType
	if err := json.NewDecoder(resList.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != at.ID {
		t.Fatalf("list: %+v", list)
	}
}

func TestAPI_accessTypeCreate_unknownField(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/api/v1/domains/"+dom.ID+"/access-types", "application/json", strings.NewReader(`{"title":"x","bit":"1","extra":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_permissionListGet_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	resEmpty, err := http.Get(base + "/permissions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resEmpty.Body.Close() }()
	if resEmpty.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resEmpty.StatusCode)
	}
	var empty []store.Permission
	if err := json.NewDecoder(resEmpty.Body).Decode(&empty); err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("want empty, got %+v", empty)
	}

	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p1","resource_id":%q,"access_mask":"0x3"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	if perm.ID == "" || perm.AccessMask != 3 {
		t.Fatalf("permission: %+v", perm)
	}

	resList, err := http.Get(base + "/permissions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resList.Body.Close() }()
	if resList.StatusCode != http.StatusOK {
		t.Fatalf("list status %d", resList.StatusCode)
	}
	var list []store.Permission
	if err := json.NewDecoder(resList.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != perm.ID {
		t.Fatalf("list: %+v", list)
	}

	resGet, err := http.Get(base + "/permissions/" + perm.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resGet.Body.Close() }()
	if resGet.StatusCode != http.StatusOK {
		t.Fatalf("get status %d", resGet.StatusCode)
	}
	var got store.Permission
	if err := json.NewDecoder(resGet.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != perm.ID || got.AccessMask != 3 {
		t.Fatalf("got %+v", got)
	}

	res404, err := http.Get(base + "/permissions/" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res404.Body.Close() }()
	if res404.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res404.StatusCode)
	}
}

func TestAPI_membershipPostDelete_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}

	addURL := base + "/users/" + user.ID + "/groups/" + g.ID
	reqPost, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPost.Body.Close() }()
	if resPost.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resPost.Body)
		t.Fatalf("POST membership want 204, got %d: %s", resPost.StatusCode, b)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel.Body.Close() }()
	if resDel.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resDel.Body)
		t.Fatalf("DELETE membership want 204, got %d: %s", resDel.StatusCode, b)
	}

	reqDel2, err := http.NewRequest(http.MethodDelete, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel2, err := http.DefaultClient.Do(reqDel2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel2.Body.Close() }()
	if resDel2.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE want 404, got %d", resDel2.StatusCode)
	}
}

func TestAPI_addUserToGroup_unknownUser(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	addURL := base + "/users/" + uuid.NewString() + "/groups/" + g.ID
	reqPost, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_userPermissionGrantRevoke_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}

	grantURL := base + "/users/" + user.ID + "/permissions/" + perm.ID
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPost.Body.Close() }()
	if resPost.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resPost.Body)
		t.Fatalf("POST grant want 204, got %d: %s", resPost.StatusCode, b)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel.Body.Close() }()
	if resDel.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resDel.Body)
		t.Fatalf("DELETE revoke want 204, got %d: %s", resDel.StatusCode, b)
	}

	reqDel2, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel2, err := http.DefaultClient.Do(reqDel2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel2.Body.Close() }()
	if resDel2.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE want 404, got %d", resDel2.StatusCode)
	}
}

func TestAPI_grantUserPermission_unknownPermission(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/users/" + user.ID + "/permissions/" + uuid.NewString()
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_groupPermissionGrantRevoke_notFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x2"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}

	grantURL := base + "/groups/" + g.ID + "/permissions/" + perm.ID
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resPost.Body.Close() }()
	if resPost.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resPost.Body)
		t.Fatalf("POST group grant want 204, got %d: %s", resPost.StatusCode, b)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel.Body.Close() }()
	if resDel.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resDel.Body)
		t.Fatalf("DELETE group revoke want 204, got %d: %s", resDel.StatusCode, b)
	}

	reqDel2, err := http.NewRequest(http.MethodDelete, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resDel2, err := http.DefaultClient.Do(reqDel2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resDel2.Body.Close() }()
	if resDel2.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE want 404, got %d", resDel2.StatusCode)
	}
}

func TestAPI_grantGroupPermission_unknownPermission(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/groups/" + g.ID + "/permissions/" + uuid.NewString()
	reqPost, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_addUserToGroup_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var group store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &group); err != nil {
		t.Fatal(err)
	}
	addURL := base + "/users/" + user.ID + "/groups/" + group.ID
	req1, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first add: want 204, got %d", res1.StatusCode)
	}
	req2, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate add: want 409, got %d", res2.StatusCode)
	}
}

func TestAPI_grantUserPermission_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/users/" + user.ID + "/permissions/" + perm.ID
	req1, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first grant: want 204, got %d", res1.StatusCode)
	}
	req2, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate grant: want 409, got %d", res2.StatusCode)
	}
}

func TestAPI_grantGroupPermission_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var group store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &group); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x1"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantURL := base + "/groups/" + group.ID + "/permissions/" + perm.ID
	req1, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first grant: want 204, got %d", res1.StatusCode)
	}
	req2, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate grant: want 409, got %d", res2.StatusCode)
	}
}

func TestAPI_authzMasks_happyPath(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var g store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"g"}`), &g); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"access_mask":"0x5"}`, resource.ID)
	var perm store.Permission
	if err := json.Unmarshal(mustPostJSON201(t, base+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}

	addURL := base + "/users/" + user.ID + "/groups/" + g.ID
	reqMem, err := http.NewRequest(http.MethodPost, addURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resMem, err := http.DefaultClient.Do(reqMem)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resMem.Body.Close() }()
	if resMem.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resMem.Body)
		t.Fatalf("membership want 204, got %d: %s", resMem.StatusCode, b)
	}

	grantURL := base + "/groups/" + g.ID + "/permissions/" + perm.ID
	reqGr, err := http.NewRequest(http.MethodPost, grantURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resGr, err := http.DefaultClient.Do(reqGr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resGr.Body.Close() }()
	if resGr.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resGr.Body)
		t.Fatalf("group grant want 204, got %d: %s", resGr.StatusCode, b)
	}

	q := fmt.Sprintf("%s/authz/masks?user_id=%s&resource_id=%s", base, user.ID, resource.ID)
	res, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("masks want 200, got %d: %s", res.StatusCode, b)
	}
	var out struct {
		Masks []uint64 `json:"masks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Masks) != 1 || out.Masks[0] != 5 {
		t.Fatalf("masks: %+v", out.Masks)
	}
}

func TestAPI_authzMasks_emptyWithoutGrants(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	var user store.User
	if err := json.Unmarshal(mustPostJSON201(t, base+"/users", `{"title":"u"}`), &user); err != nil {
		t.Fatal(err)
	}
	var resource store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"r"}`), &resource); err != nil {
		t.Fatal(err)
	}
	q := fmt.Sprintf("%s/authz/masks?user_id=%s&resource_id=%s", base, user.ID, resource.ID)
	res, err := http.Get(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 200, got %d: %s", res.StatusCode, b)
	}
	var out struct {
		Masks []uint64 `json:"masks"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Masks) != 0 {
		t.Fatalf("want empty masks, got %+v", out.Masks)
	}
}

// --- empty-list tests to cover the nil→[] fallback branches ---

func TestAPI_domainList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	res, err := http.Get(ts.URL + "/api/v1/domains")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var list []store.Domain
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d items", len(list))
	}
}

func TestAPI_groupList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domID + "/groups")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var list []store.Group
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d items", len(list))
	}
}

func TestAPI_resourceList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domID + "/resources")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var list []store.Resource
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d items", len(list))
	}
}

func TestAPI_permissionList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domID + "/permissions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var list []store.Permission
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d items", len(list))
	}
}

func TestAPI_accessTypeList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domID + "/access-types")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var list []store.AccessType
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d items", len(list))
	}
}

// mustCreateDomain is a test helper that creates a domain and returns its ID.
func mustCreateDomain(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	b := mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"test-domain"}`)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}

// --- duplicate-create 409 tests ---

func TestAPI_accessTypeCreate_duplicateBit(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)

	res, err := http.Post(base+"/access-types", "application/json", strings.NewReader(`{"title":"write","bit":"0x1"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("duplicate bit want 409, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_domainCreate_storeErrorClassified(t *testing.T) {
	ts := newBrokenTestAPI(t)
	res, err := http.Post(ts.URL+"/api/v1/domains", "application/json", strings.NewReader(`{"title":"d"}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("broken store want 500, got %d", res.StatusCode)
	}
}

// --- writeStoreErr unit tests ---

func TestWriteStoreErr_allCases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", store.ErrNotFound, http.StatusNotFound},
		{"fk violation", store.ErrFKViolation, http.StatusBadRequest},
		{"invalid input", store.ErrInvalidInput, http.StatusBadRequest},
		{"conflict", store.ErrConflict, http.StatusConflict},
		{"generic", fmt.Errorf("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeStoreErr(w, nil, tt.err)
			if w.Code != tt.want {
				t.Fatalf("writeStoreErr(%v) = %d, want %d", tt.err, w.Code, tt.want)
			}
		})
	}
}

// --- store-error tests using a broken (closed-DB) store ---

func newBrokenTestAPI(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sqlstore.Open("file:" + filepath.Join(t.TempDir(), "broken.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlstore.MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	st := sqlstore.New(db)
	_ = db.Close()
	srv := &Server{Store: st}
	ts := httptest.NewServer(srv.Router(nil, nil))
	t.Cleanup(ts.Close)
	return ts
}

func TestAPI_storeErrors(t *testing.T) {
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

func TestAPI_domainGetPatchDelete(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"orig"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	res, err := http.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET domain want 200, got %d: %s", res.StatusCode, b)
	}
	var got store.Domain
	if err := json.Unmarshal(b, &got); err != nil || got.Title != "orig" {
		t.Fatalf("domain: %+v err=%v", got, err)
	}

	reqPatch, err := http.NewRequest(http.MethodPatch, base, strings.NewReader(`{"title":"renamed"}`))
	if err != nil {
		t.Fatal(err)
	}
	reqPatch.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(reqPatch)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PATCH domain want 200, got %d: %s", res.StatusCode, b)
	}
	if err := json.Unmarshal(b, &got); err != nil || got.Title != "renamed" {
		t.Fatalf("patched: %+v", got)
	}

	reqDel, err := http.NewRequest(http.MethodDelete, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE domain want 204, got %d", res.StatusCode)
	}
}

func TestAPI_domainDelete_blockedByUser(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	_ = mustPostJSON201(t, base+"/users", `{"title":"u"}`)

	reqDel, err := http.NewRequest(http.MethodDelete, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("DELETE domain with user want 400, got %d: %s", res.StatusCode, b)
	}
}

func TestAPI_userResourcePermissionPatchDelete(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID

	uBody := mustPostJSON201(t, base+"/users", `{"title":"u"}`)
	var u store.User
	if err := json.Unmarshal(uBody, &u); err != nil {
		t.Fatal(err)
	}
	reqUP, _ := http.NewRequest(http.MethodPatch, base+"/users/"+u.ID, strings.NewReader(`{"title":"v"}`))
	reqUP.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(reqUP)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("user patch: %d %s", res.StatusCode, b)
	}

	rBody := mustPostJSON201(t, base+"/resources", `{"title":"r"}`)
	var resrc store.Resource
	if err := json.Unmarshal(rBody, &resrc); err != nil {
		t.Fatal(err)
	}
	reqRP, _ := http.NewRequest(http.MethodPatch, base+"/resources/"+resrc.ID, strings.NewReader(`{"title":"r2"}`))
	reqRP.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(reqRP)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("resource patch: %d %s", res.StatusCode, b)
	}

	atBody := mustPostJSON201(t, base+"/access-types", `{"title":"read","bit":"0x1"}`)
	var at store.AccessType
	if err := json.Unmarshal(atBody, &at); err != nil {
		t.Fatal(err)
	}
	reqAT, _ := http.NewRequest(http.MethodPatch, base+"/access-types/"+at.ID, strings.NewReader(`{"title":"READ"}`))
	reqAT.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(reqAT)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("access type patch: %d %s", res.StatusCode, b)
	}

	pBody := mustPostJSON201(t, base+"/permissions", `{"title":"p","resource_id":"`+resrc.ID+`","access_mask":"0x3"}`)
	var perm store.Permission
	if err := json.Unmarshal(pBody, &perm); err != nil {
		t.Fatal(err)
	}
	reqPP, _ := http.NewRequest(http.MethodPatch, base+"/permissions/"+perm.ID, strings.NewReader(`{"title":"perm2"}`))
	reqPP.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(reqPP)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("permission patch: %d %s", res.StatusCode, b)
	}

	reqPD, _ := http.NewRequest(http.MethodDelete, base+"/permissions/"+perm.ID, nil)
	res, err = http.DefaultClient.Do(reqPD)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("permission delete: %d", res.StatusCode)
	}

	reqRD, _ := http.NewRequest(http.MethodDelete, base+"/resources/"+resrc.ID, nil)
	res, err = http.DefaultClient.Do(reqRD)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("resource delete: %d", res.StatusCode)
	}

	reqATD, _ := http.NewRequest(http.MethodDelete, base+"/access-types/"+at.ID, nil)
	res, err = http.DefaultClient.Do(reqATD)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("access type delete: %d", res.StatusCode)
	}

	reqUD, _ := http.NewRequest(http.MethodDelete, base+"/users/"+u.ID, nil)
	res, err = http.DefaultClient.Do(reqUD)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("user delete: %d", res.StatusCode)
	}
}

func TestAPI_groupPatchDelete(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	g1 := json.RawMessage(mustPostJSON201(t, base+"/groups", `{"title":"g1"}`))
	g2 := json.RawMessage(mustPostJSON201(t, base+"/groups", `{"title":"g2"}`))
	var grp1, grp2 store.Group
	_ = json.Unmarshal(g1, &grp1)
	_ = json.Unmarshal(g2, &grp2)

	reqGP, _ := http.NewRequest(http.MethodPatch, base+"/groups/"+grp2.ID,
		strings.NewReader(`{"title":"two","parent_group_id":"`+grp1.ID+`"}`))
	reqGP.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(reqGP)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("group patch: %d %s", res.StatusCode, b)
	}

	reqGD, _ := http.NewRequest(http.MethodDelete, base+"/groups/"+grp2.ID, nil)
	res, err = http.DefaultClient.Do(reqGD)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("group delete child: %d", res.StatusCode)
	}

	reqGD2, _ := http.NewRequest(http.MethodDelete, base+"/groups/"+grp1.ID, nil)
	res, err = http.DefaultClient.Do(reqGD2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("group delete parent: %d", res.StatusCode)
	}
}

func TestAPI_patchEmptyBody(t *testing.T) {
	ts, _ := newTestAPI(t)
	var dom store.Domain
	if err := json.Unmarshal(mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"d"}`), &dom); err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/api/v1/domains/" + dom.ID
	uBody := mustPostJSON201(t, base+"/users", `{"title":"u"}`)
	var u store.User
	if err := json.Unmarshal(uBody, &u); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, base+"/users/"+u.ID, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty patch want 400, got %d", res.StatusCode)
	}
}
