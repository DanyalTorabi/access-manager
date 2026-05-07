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
	var emptyEnv listResponse[store.Permission]
	if err := json.NewDecoder(resEmpty.Body).Decode(&emptyEnv); err != nil {
		t.Fatal(err)
	}
	if len(emptyEnv.Data) != 0 {
		t.Fatalf("want empty, got %+v", emptyEnv.Data)
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
	var listEnv listResponse[store.Permission]
	if err := json.NewDecoder(resList.Body).Decode(&listEnv); err != nil {
		t.Fatal(err)
	}
	if len(listEnv.Data) != 1 || listEnv.Data[0].ID != perm.ID {
		t.Fatalf("list: %+v", listEnv.Data)
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
	var env listResponse[store.Permission]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 0 {
		t.Fatalf("want empty list, got %d items", len(env.Data))
	}
}


func TestAPI_permissionList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domID

	var r store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"res"}`), &r); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"can-read", "can-write", "can-read-all"} {
		mustPostJSON201(t, base+"/permissions", fmt.Sprintf(`{"title":%q,"resource_id":%q,"access_mask":"1"}`, title, r.ID))
	}

	res, err := http.Get(base + "/permissions?search=can-read")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Permission]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}


func TestAPI_permissionList_filterByResourceID(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + domID

	var r1, r2 store.Resource
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"res1"}`), &r1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustPostJSON201(t, base+"/resources", `{"title":"res2"}`), &r2); err != nil {
		t.Fatal(err)
	}
	mustPostJSON201(t, base+"/permissions", fmt.Sprintf(`{"title":"p1","resource_id":%q,"access_mask":"1"}`, r1.ID))
	mustPostJSON201(t, base+"/permissions", fmt.Sprintf(`{"title":"p2","resource_id":%q,"access_mask":"2"}`, r1.ID))
	mustPostJSON201(t, base+"/permissions", fmt.Sprintf(`{"title":"p3","resource_id":%q,"access_mask":"4"}`, r2.ID))

	res, err := http.Get(base + "/permissions?resource_id=" + r1.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Permission]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("want 2 for r1, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
}


func TestAPI_permissionList_sortByResourceID(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)

	resA := mustCreateResource(t, ts, domainID, "Resource A")
	resB := mustCreateResource(t, ts, domainID, "Resource B")

	mustPostJSON201(t, ts.URL+"/api/v1/domains/"+domainID+"/permissions",
		fmt.Sprintf(`{"title":"perm-b","resource_id":%q,"access_mask":"1"}`, resB))
	mustPostJSON201(t, ts.URL+"/api/v1/domains/"+domainID+"/permissions",
		fmt.Sprintf(`{"title":"perm-a","resource_id":%q,"access_mask":"2"}`, resA))

	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/permissions?sort=resource_id&order=asc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d: %s", res.StatusCode, b)
	}
	var env listResponse[store.Permission]
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("want 2, got %d", len(env.Data))
	}
	if env.Data[0].ResourceID > env.Data[1].ResourceID {
		t.Fatalf("expected ascending resource_id order: %s > %s", env.Data[0].ResourceID, env.Data[1].ResourceID)
	}
	if env.Meta.Sort != "resource_id" || env.Meta.Order != "asc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}
}


func TestAPI_permissionList_invalidSort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domainID := mustCreateDomain(t, ts)

	res, err := http.Get(ts.URL + "/api/v1/domains/" + domainID + "/permissions?sort=access_mask")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}


func TestAPI_permissionPatch_invalidMask(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + dom
	rBody := mustPostJSON201(t, base+"/resources", `{"title":"r"}`)
	var resrc store.Resource
	if err := json.Unmarshal(rBody, &resrc); err != nil {
		t.Fatal(err)
	}
	pBody := mustPostJSON201(t, base+"/permissions", `{"title":"p","resource_id":"`+resrc.ID+`","access_mask":"0x1"}`)
	var perm store.Permission
	if err := json.Unmarshal(pBody, &perm); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, base+"/permissions/"+perm.ID,
		strings.NewReader(`{"access_mask":"bad"}`))
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


func TestAPI_permissionPatch_maskAndResource(t *testing.T) {
	ts, _ := newTestAPI(t)
	dom := mustCreateDomain(t, ts)
	base := ts.URL + "/api/v1/domains/" + dom
	r1Body := mustPostJSON201(t, base+"/resources", `{"title":"r1"}`)
	var r1 store.Resource
	if err := json.Unmarshal(r1Body, &r1); err != nil {
		t.Fatal(err)
	}
	r2Body := mustPostJSON201(t, base+"/resources", `{"title":"r2"}`)
	var r2 store.Resource
	if err := json.Unmarshal(r2Body, &r2); err != nil {
		t.Fatal(err)
	}
	pBody := mustPostJSON201(t, base+"/permissions", `{"title":"p","resource_id":"`+r1.ID+`","access_mask":"0x1"}`)
	var perm store.Permission
	if err := json.Unmarshal(pBody, &perm); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, base+"/permissions/"+perm.ID,
		strings.NewReader(`{"access_mask":"0xFF","resource_id":"`+r2.ID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", res.StatusCode, b)
	}
	var got store.Permission
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.AccessMask != 0xFF || got.ResourceID != r2.ID {
		t.Fatalf("got mask=%#x resource=%s", got.AccessMask, got.ResourceID)
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

