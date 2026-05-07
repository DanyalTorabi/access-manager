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
	"github.com/google/uuid"
)

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
	var emptyEnv listResponse[store.Group]
	if err := json.NewDecoder(resEmpty.Body).Decode(&emptyEnv); err != nil {
		t.Fatal(err)
	}
	if len(emptyEnv.Data) != 0 {
		t.Fatalf("want empty groups, got %+v", emptyEnv.Data)
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
	var listEnv listResponse[store.Group]
	if err := json.NewDecoder(resList.Body).Decode(&listEnv); err != nil {
		t.Fatal(err)
	}
	if len(listEnv.Data) != 1 || listEnv.Data[0].ID != g.ID {
		t.Fatalf("list: %+v", listEnv.Data)
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

func TestAPI_groupList_empty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "test-domain")
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domID + "/groups")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var env listResponse[store.Group]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %d items", len(env.Data))
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
	if err := json.Unmarshal(g1, &grp1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(g2, &grp2); err != nil {
		t.Fatal(err)
	}

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

func TestAPI_groupList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + domID
	for _, title := range []string{"Admins", "Editors", "Admin-sub"} {
		mustPostJSON201(t, base+"/groups", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(base + "/groups?search=admin")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Group]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_groupList_filterByParentGroupID(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + domID

	var parent store.Group
	if err := json.Unmarshal(mustPostJSON201(t, base+"/groups", `{"title":"parent"}`), &parent); err != nil {
		t.Fatal(err)
	}
	mustPostJSON201(t, base+"/groups", fmt.Sprintf(`{"title":"child1","parent_group_id":%q}`, parent.ID))
	mustPostJSON201(t, base+"/groups", fmt.Sprintf(`{"title":"child2","parent_group_id":%q}`, parent.ID))
	mustPostJSON201(t, base+"/groups", `{"title":"other-root"}`)

	res, err := http.Get(base + "/groups?parent_group_id=" + parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Group]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2 children, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}

func TestAPI_groupList_sortDesc(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + domainID

	for _, title := range []string{"Admins", "Editors", "Viewers"} {
		mustPostJSON201(t, base+"/groups", fmt.Sprintf(`{"title":%q}`, title))
	}

	res, err := http.Get(base + "/groups?sort=title&order=desc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Group]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 3 {
		t.Fatalf("want 3, got %d", len(env.Data))
	}
	if env.Data[0].Title != "Viewers" || env.Data[2].Title != "Admins" {
		t.Fatalf("order: got %q %q %q", env.Data[0].Title, env.Data[1].Title, env.Data[2].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "desc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}

func TestAPI_groupList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := seedDomain(t, ts, "test-domain")
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/groups?sort=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

func TestAPI_groupList_invalidOrder(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := seedDomain(t, ts, "test-domain")
	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/groups?order=bad")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

func TestAPI_groupPatch_parentGroupIDInvalid(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + dom
	gBody := mustPostJSON201(t, base+"/groups", `{"title":"g"}`)
	var grp store.Group
	if err := json.Unmarshal(gBody, &grp); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, base+"/groups/"+grp.ID,
		strings.NewReader(`{"parent_group_id":123}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for numeric parent_group_id, got %d", res.StatusCode)
	}
}

func TestAPI_groupPatch_clearParent(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := seedDomain(t, ts, "test-domain")
	base := ts.URL + "/api/v1/domains/" + dom
	g1Body := mustPostJSON201(t, base+"/groups", `{"title":"g1"}`)
	g2Body := mustPostJSON201(t, base+"/groups", `{"title":"g2"}`)
	var g1, g2 store.Group
	if err := json.Unmarshal(g1Body, &g1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(g2Body, &g2); err != nil {
		t.Fatal(err)
	}

	reqSet, _ := http.NewRequest(http.MethodPatch, base+"/groups/"+g2.ID,
		strings.NewReader(`{"parent_group_id":"`+g1.ID+`"}`))
	reqSet.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(reqSet)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("set parent: want 200, got %d", res.StatusCode)
	}

	reqClear, _ := http.NewRequest(http.MethodPatch, base+"/groups/"+g2.ID,
		strings.NewReader(`{"parent_group_id":null}`))
	reqClear.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(reqClear)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("clear parent: want 200, got %d: %s", res.StatusCode, b)
	}
	var got store.Group
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ParentGroupID != nil {
		t.Fatalf("parent should be nil, got %v", got.ParentGroupID)
	}
}
