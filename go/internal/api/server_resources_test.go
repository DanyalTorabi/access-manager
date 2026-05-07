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
	var emptyEnv listResponse[store.Resource]
	if err := json.NewDecoder(resEmpty.Body).Decode(&emptyEnv); err != nil {
		t.Fatal(err)
	}
	if len(emptyEnv.Data) != 0 {
		t.Fatalf("want empty, got %+v", emptyEnv.Data)
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
	var listEnv listResponse[store.Resource]
	if err := json.NewDecoder(resList.Body).Decode(&listEnv); err != nil {
		t.Fatal(err)
	}
	if len(listEnv.Data) != 1 || listEnv.Data[0].ID != r.ID {
		t.Fatalf("list: %+v", listEnv.Data)
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
	var env listResponse[store.Resource]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %d items", len(env.Data))
	}
}


func TestAPI_resourceList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domID
	for _, title := range []string{"Document", "Image", "Documentation"} {
		mustPostJSON201(t, base+"/resources", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(base + "/resources?search=doc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Resource]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}


func TestAPI_resourceList_sortDesc(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domainID

	for _, title := range []string{"Docs", "Files", "Settings"} {
		mustPostJSON201(t, base+"/resources", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(base + "/resources?sort=title&order=desc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Resource]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("want 3, got %d", len(env.Data))
	}
	if env.Data[0].Title != "Settings" || env.Data[2].Title != "Docs" {
		t.Fatalf("order: got %q %q %q", env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "desc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}


func TestAPI_resourceList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/resources?sort=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}


func TestAPI_resourceList_invalidOrder(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/resources?order=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}


func TestAPI_resourcePatch_emptyAndNotFound(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + dom
	rBody := mustPostJSON201(t, base+"/resources", `{"title":"r"}`)
	var resrc store.Resource
	if err := json.Unmarshal(rBody, &resrc); err != nil {
		t.Fatal(err)
	}

	reqEmpty, _ := http.NewRequest(http.MethodPatch, base+"/resources/"+resrc.ID,
		strings.NewReader(`{}`))
	reqEmpty.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(reqEmpty)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty patch: want 400, got %d", res.StatusCode)
	}

	reqNF, _ := http.NewRequest(http.MethodPatch, base+"/resources/"+uuid.NewString(),
		strings.NewReader(`{"title":"x"}`))
	reqNF.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(reqNF)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("not found patch: want 404, got %d", res.StatusCode)
	}
}

