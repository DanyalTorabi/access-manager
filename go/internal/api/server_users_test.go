package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
	var env listResponse[store.User]
	if err := json.NewDecoder(res2.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %+v", env.Data)
	}
	if env.Meta.Total != 0 {
		t.Fatalf("meta.total: want 0, got %d", env.Meta.Total)
	}
}

func TestAPI_userList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + domID
	for _, title := range []string{"Alice", "Bob", "Alicia"} {
		mustPostJSON201(t, base+"/users", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(base + "/users?search=ali")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.User]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_userList_sortDesc(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + domainID

	for _, title := range []string{"Alice", "Bob", "Charlie"} {
		mustPostJSON201(t, base+"/users", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(base + "/users?sort=title&order=desc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.User]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("want 3, got %d", len(env.Data))
	}
	if env.Data[0].Title != "Charlie" || env.Data[2].Title != "Alice" {
		t.Fatalf("order: got %q %q %q", env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "desc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}

func TestAPI_userList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := seedDomain(t, ts, "test-domain")
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/users?sort=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

func TestAPI_userList_invalidOrder(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := seedDomain(t, ts, "test-domain")
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/users?order=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
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
