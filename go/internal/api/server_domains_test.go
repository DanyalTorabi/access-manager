package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/logger"
	"github.com/dtorabi/access-manager/internal/store"
)

// NOTE: Tests that call logger.Init mutate the package-level logger pointer.
// This is safe only because no test in this file uses t.Parallel().
// Do NOT add t.Parallel() without first switching to a logger-injectable
// Server field or an atomic pointer. Tracked on #47 (T36 follow-ups).

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
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res2.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].ID != created.ID {
		t.Fatalf("list data: %+v", env.Data)
	}
	if env.Meta.Total != 1 {
		t.Fatalf("meta.total: want 1, got %d", env.Meta.Total)
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
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %d items", len(env.Data))
	}
	if env.Meta.Total != 0 {
		t.Fatalf("meta.total: want 0, got %d", env.Meta.Total)
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

func TestAPI_domainPatch_malformedJSON(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/domains/"+dom,
		strings.NewReader(`{broken`))
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
}

// --- pagination tests ---

func TestAPI_domainList_pagination(t *testing.T) {
	ts, _ := newTestAPI(t)
	for i := 0; i < 5; i++ {
		title := fmt.Sprintf("dom-%c", 'a'+i)
		mustPostJSON201(t, ts.URL+"/api/v1/domains", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(ts.URL + "/api/v1/domains?offset=1&limit=2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 5 {
		t.Fatalf("meta.total: want 5, got %d", env.Meta.Total)
	}
	if env.Meta.Offset != 1 || env.Meta.Limit != 2 {
		t.Fatalf("meta: offset=%d limit=%d", env.Meta.Offset, env.Meta.Limit)
	}
	if len(env.Data) != 2 {
		t.Fatalf("data len: want 2, got %d", len(env.Data))
	}
}

func TestAPI_domainList_defaultPagination(t *testing.T) {
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"one"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains")
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
	if env.Meta.Offset != 0 || env.Meta.Limit != 20 {
		t.Fatalf("defaults: offset=%d limit=%d", env.Meta.Offset, env.Meta.Limit)
	}
	if env.Meta.Total != 1 || len(env.Data) != 1 {
		t.Fatalf("total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_domainList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	for _, title := range []string{"Alpha", "Beta", "Alphabet"} {
		mustPostJSON201(t, ts.URL+"/api/v1/domains", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(ts.URL + "/api/v1/domains?search=alph")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2 results, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_domainList_searchNoMatch(t *testing.T) {
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"Alpha"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains?search=zzz")
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
	if env.Meta.Total != 0 || len(env.Data) != 0 {
		t.Fatalf("want 0 results, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_domainList_searchEmptyIgnored(t *testing.T) {
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"one"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains?search=")
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
		t.Fatalf("empty search should return all, got total=%d", env.Meta.Total)
	}
}

func TestAPI_domainList_searchWithPagination(t *testing.T) {
	ts, _ := newTestAPI(t)
	for i := 0; i < 5; i++ {
		mustPostJSON201(t, ts.URL+"/api/v1/domains", fmt.Sprintf(`{"title":"test-%c"}`, 'a'+i))
	}
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"other"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains?search=test&limit=2&offset=0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 5 {
		t.Fatalf("total should be 5 (all matching), got %d", env.Meta.Total)
	}
	if len(env.Data) != 2 {
		t.Fatalf("page size should be 2, got %d", len(env.Data))
	}
}

func TestAPI_domainList_searchEscapesWildcards(t *testing.T) {
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"100% done"}`)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"normal"}`)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"test_case"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains?search=%25")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 || len(env.Data) != 1 {
		t.Fatalf("search for literal %%: want 1 result, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_domainList_searchType(t *testing.T) {
	ts, _ := newTestAPI(t)
	for _, title := range []string{"Alpha", "Alphabet", "Beta"} {
		mustPostJSON201(t, ts.URL+"/api/v1/domains", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(ts.URL + "/api/v1/domains?search=Alpha&search_type=starts_with")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("starts_with Alpha: want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}

	res2, err := http.Get(ts.URL + "/api/v1/domains?search=bet&search_type=ends_with")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res2.Body.Close() }()
	if res2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("ends_with status %d: %s", res2.StatusCode, b)
	}
	var env2 listResponse[store.Domain]
	if err := json.NewDecoder(res2.Body).Decode(&env2); err != nil {
		t.Fatal(err)
	}
	if env2.Meta.Total != 1 {
		t.Fatalf("ends_with bet: want 1, got total=%d", env2.Meta.Total)
	}

	res3, err := http.Get(ts.URL + "/api/v1/domains?search=foo&search_type=invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res3.Body.Close() }()
	if res3.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid search_type: want 400, got %d", res3.StatusCode)
	}
}

func TestAPI_domainList_sortMeta(t *testing.T) {
	ts, _ := newTestAPI(t)
	mustPostJSON201(t, ts.URL+"/api/v1/domains", `{"title":"one"}`)

	res, err := http.Get(ts.URL + "/api/v1/domains")
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
	if env.Meta.Sort != "title" {
		t.Fatalf("meta.sort: want title, got %q", env.Meta.Sort)
	}
	if env.Meta.Order != "asc" {
		t.Fatalf("meta.order: want asc, got %q", env.Meta.Order)
	}
}

func TestAPI_domainList_sortDesc(t *testing.T) {
	ts, _ := newTestAPI(t)
	for _, title := range []string{"Alpha", "Beta", "Charlie"} {
		mustPostJSON201(t, ts.URL+"/api/v1/domains", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(ts.URL + "/api/v1/domains?sort=title&order=desc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Domain]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("want 3 items, got %d", len(env.Data))
	}
	if env.Data[0].Title != "Charlie" || env.Data[2].Title != "Alpha" {
		t.Fatalf("order: got %q, %q, %q", env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "desc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}

func TestAPI_domainList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)

	res, err := http.Get(ts.URL + "/api/v1/domains?sort=unknown_field")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

func TestAPI_domainList_invalidOrder(t *testing.T) {
	ts, _ := newTestAPI(t)

	res, err := http.Get(ts.URL + "/api/v1/domains?order=random")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}
