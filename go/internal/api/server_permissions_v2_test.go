package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

// seedAccessTypeV2 creates an access type via v2 (auto-allocates bit).
func seedAccessTypeV2(t *testing.T, ts *httptest.Server, domID, title string) store.AccessType {
	t.Helper()
	var at store.AccessType
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/access-types", fmt.Sprintf(`{"title":%q}`, title)), &at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestAPI_v2_permissionCreate_withTitles(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read","write"]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	if len(perm.Permissions) != 2 || perm.Permissions[0] != "read" || perm.Permissions[1] != "write" {
		t.Fatalf("permissions: want [read write], got %v", perm.Permissions)
	}
	if perm.ResourceID != resID {
		t.Fatalf("resource_id: want %q, got %q", resID, perm.ResourceID)
	}
}

func TestAPI_v2_permissionCreate_unknownTitle_400(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["nope"]}`, resID)
	mustPostJSON(t, domainBaseV2(ts, domID)+"/permissions", body, http.StatusBadRequest)
}

func TestAPI_v2_permissionCreate_emptyPermissions(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":[]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	if len(perm.Permissions) != 0 {
		t.Fatalf("want empty permissions, got %v", perm.Permissions)
	}
}

func TestAPI_v2_permissionGet(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read"]}`, resID)
	var created permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &created); err != nil {
		t.Fatal(err)
	}

	b := mustGet(t, domainBaseV2(ts, domID)+"/permissions/"+created.ID, http.StatusOK)
	var got permissionResponseV2
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Permissions) != 1 || got.Permissions[0] != "read" {
		t.Fatalf("GET v2: want [read], got %v", got.Permissions)
	}
}

func TestAPI_v2_permissionList(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions",
		fmt.Sprintf(`{"title":"p1","resource_id":%q,"permissions":["read"]}`, resID))
	mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions",
		fmt.Sprintf(`{"title":"p2","resource_id":%q,"permissions":["write"]}`, resID))

	b := mustGet(t, domainBaseV2(ts, domID)+"/permissions", http.StatusOK)
	var env listResponse[permissionResponseV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("want 2 permissions, got %d", len(env.Data))
	}
	for _, p := range env.Data {
		if len(p.Permissions) != 1 {
			t.Fatalf("each permission should have 1 title, got %v for %q", p.Permissions, p.Title)
		}
	}
}

func TestAPI_v2_permissionPatch_updatePermissions(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read"]}`, resID)
	var created permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &created); err != nil {
		t.Fatal(err)
	}

	patch := mustDoRequest(t, http.MethodPatch,
		domainBaseV2(ts, domID)+"/permissions/"+created.ID,
		`{"permissions":["read","write"]}`,
		http.StatusOK)
	var patched permissionResponseV2
	if err := json.Unmarshal(patch, &patched); err != nil {
		t.Fatal(err)
	}
	if len(patched.Permissions) != 2 || patched.Permissions[0] != "read" || patched.Permissions[1] != "write" {
		t.Fatalf("patched permissions: want [read write], got %v", patched.Permissions)
	}
}

func TestAPI_v2_permissionPatch_unknownTitle_400(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read"]}`, resID)
	var created permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &created); err != nil {
		t.Fatal(err)
	}

	mustDoRequest(t, http.MethodPatch,
		domainBaseV2(ts, domID)+"/permissions/"+created.ID,
		`{"permissions":["nope"]}`,
		http.StatusBadRequest)
}

// TestAPI_v2_permission_v1CompatibilityRegression verifies that a permission
// created via v1 with a numeric mask is readable via v2 as a title array, and
// that the same permission retrieved via v1 still returns a numeric access_mask.
func TestAPI_v2_permission_v1CompatibilityRegression(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	// Register access type with explicit bit via v1.
	seedAccessType(t, ts, domID, "read", "1")

	// Create permission via v1 with numeric mask.
	permID := seedPermission(t, ts, domID, "p", resID, "1")

	// GET via v2 → title array.
	b := mustGet(t, domainBaseV2(ts, domID)+"/permissions/"+permID, http.StatusOK)
	var v2resp permissionResponseV2
	if err := json.Unmarshal(b, &v2resp); err != nil {
		t.Fatal(err)
	}
	if len(v2resp.Permissions) != 1 || v2resp.Permissions[0] != "read" {
		t.Fatalf("v2 GET on v1-created permission: want [read], got %v", v2resp.Permissions)
	}

	// GET via v1 → numeric access_mask still present.
	b2 := mustGet(t, domainBase(ts, domID)+"/permissions/"+permID, http.StatusOK)
	var v1resp store.Permission
	if err := json.Unmarshal(b2, &v1resp); err != nil {
		t.Fatal(err)
	}
	if v1resp.AccessMask != 1 {
		t.Fatalf("v1 GET on permission: want AccessMask=1, got %d", v1resp.AccessMask)
	}
}

// --- List and PATCH tests (T70) ---

// TestAPI_v2_permissionList_search verifies that the V2 permission list
// supports ?search= (title substring) and ?resource_id= filtering.
func TestAPI_v2_permissionList_search(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	for _, title := range []string{"can-read", "can-write", "can-read-all"} {
		mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions",
			fmt.Sprintf(`{"title":%q,"resource_id":%q,"permissions":["read"]}`, title, resID))
	}

	// Search by title substring.
	b := mustGet(t, domainBaseV2(ts, domID)+"/permissions?search=can-read", http.StatusOK)
	var env listResponse[permissionResponseV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 || len(env.Data) != 2 {
		t.Fatalf("search 'can-read': want 2, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
	for _, p := range env.Data {
		if !strings.Contains(p.Title, "can-read") {
			t.Fatalf("search result title %q does not contain 'can-read'", p.Title)
		}
	}

	// Filter by resource_id.
	res2ID := seedResource(t, ts, domID, "res2")
	mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions",
		fmt.Sprintf(`{"title":"other","resource_id":%q,"permissions":["read"]}`, res2ID))

	bRes := mustGet(t, domainBaseV2(ts, domID)+"/permissions?resource_id="+resID, http.StatusOK)
	var envRes listResponse[permissionResponseV2]
	if err := json.Unmarshal(bRes, &envRes); err != nil {
		t.Fatal(err)
	}
	if envRes.Meta.Total != 3 {
		t.Fatalf("filter resource_id: want total=3, got %d", envRes.Meta.Total)
	}
	for _, p := range envRes.Data {
		if p.ResourceID != resID {
			t.Fatalf("filter resource_id: unexpected resource_id %q", p.ResourceID)
		}
	}
}

// TestAPI_v2_permissionList_sort verifies that the V2 permission list
// supports ?sort= and ?order= query parameters.
func TestAPI_v2_permissionList_sort(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resA := seedResource(t, ts, domID, "Resource A")
	resB := seedResource(t, ts, domID, "Resource B")
	seedAccessTypeV2(t, ts, domID, "read")

	mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions",
		fmt.Sprintf(`{"title":"perm-b","resource_id":%q,"permissions":["read"]}`, resB))
	mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions",
		fmt.Sprintf(`{"title":"perm-a","resource_id":%q,"permissions":["read"]}`, resA))

	// Sort by title asc.
	b := mustGet(t, domainBaseV2(ts, domID)+"/permissions?sort=title&order=asc", http.StatusOK)
	var env listResponse[permissionResponseV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("want 2 permissions, got %d", len(env.Data))
	}
	if env.Data[0].Title != "perm-a" || env.Data[1].Title != "perm-b" {
		t.Fatalf("sort title asc: want [perm-a perm-b], got [%q %q]",
			env.Data[0].Title, env.Data[1].Title)
	}
	if env.Meta.Sort != "title" || env.Meta.Order != "asc" {
		t.Fatalf("meta: sort=%q order=%q", env.Meta.Sort, env.Meta.Order)
	}

	// Sort by title desc.
	bDesc := mustGet(t, domainBaseV2(ts, domID)+"/permissions?sort=title&order=desc", http.StatusOK)
	var envDesc listResponse[permissionResponseV2]
	if err := json.Unmarshal(bDesc, &envDesc); err != nil {
		t.Fatal(err)
	}
	if envDesc.Data[0].Title != "perm-b" {
		t.Fatalf("sort title desc: want perm-b first, got %q", envDesc.Data[0].Title)
	}
}

// TestAPI_v2_permissionPatch_titleOnly patches only the title field via V2
// and verifies the V1 GET still returns a consistent numeric access_mask.
func TestAPI_v2_permissionPatch_titleOnly(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	body := fmt.Sprintf(`{"title":"original","resource_id":%q,"permissions":["read"]}`, resID)
	var created permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &created); err != nil {
		t.Fatal(err)
	}

	// PATCH title only via V2.
	patched := mustDoRequest(t, http.MethodPatch,
		domainBaseV2(ts, domID)+"/permissions/"+created.ID,
		`{"title":"renamed"}`,
		http.StatusOK)
	var patchedResp permissionResponseV2
	if err := json.Unmarshal(patched, &patchedResp); err != nil {
		t.Fatal(err)
	}
	if patchedResp.Title != "renamed" {
		t.Fatalf("V2 PATCH title: want 'renamed', got %q", patchedResp.Title)
	}
	// Permissions (mask) must be unchanged.
	if len(patchedResp.Permissions) != 1 || patchedResp.Permissions[0] != "read" {
		t.Fatalf("V2 PATCH title: permissions changed unexpectedly: %v", patchedResp.Permissions)
	}

	// V1 GET must reflect updated title and same numeric mask (backward compat).
	bV1 := mustGet(t, domainBase(ts, domID)+"/permissions/"+created.ID, http.StatusOK)
	var v1resp store.Permission
	if err := json.Unmarshal(bV1, &v1resp); err != nil {
		t.Fatal(err)
	}
	if v1resp.Title != "renamed" {
		t.Fatalf("V1 GET after V2 title patch: want 'renamed', got %q", v1resp.Title)
	}
	if v1resp.AccessMask != 1 {
		t.Fatalf("V1 GET after V2 title patch: access_mask changed, want 1 got %d", v1resp.AccessMask)
	}
}

// TestAPI_v2_permissionPatch_resourceOnly patches only the resource_id field via
// V2 and verifies V1 GET returns the updated resource while titles are unchanged.
func TestAPI_v2_permissionPatch_resourceOnly(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	res1ID := seedResource(t, ts, domID, "res1")
	res2ID := seedResource(t, ts, domID, "res2")
	seedAccessTypeV2(t, ts, domID, "write")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["write"]}`, res1ID)
	var created permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &created); err != nil {
		t.Fatal(err)
	}

	// PATCH resource_id only via V2.
	patchBody := fmt.Sprintf(`{"resource_id":%q}`, res2ID)
	patched := mustDoRequest(t, http.MethodPatch,
		domainBaseV2(ts, domID)+"/permissions/"+created.ID,
		patchBody,
		http.StatusOK)
	var patchedResp permissionResponseV2
	if err := json.Unmarshal(patched, &patchedResp); err != nil {
		t.Fatal(err)
	}
	if patchedResp.ResourceID != res2ID {
		t.Fatalf("V2 PATCH resource: want %q, got %q", res2ID, patchedResp.ResourceID)
	}
	// Permissions must be unchanged.
	if len(patchedResp.Permissions) != 1 || patchedResp.Permissions[0] != "write" {
		t.Fatalf("V2 PATCH resource: permissions changed unexpectedly: %v", patchedResp.Permissions)
	}

	// V1 GET must reflect updated resource_id and same numeric mask.
	bV1 := mustGet(t, domainBase(ts, domID)+"/permissions/"+created.ID, http.StatusOK)
	var v1resp store.Permission
	if err := json.Unmarshal(bV1, &v1resp); err != nil {
		t.Fatal(err)
	}
	if v1resp.ResourceID != res2ID {
		t.Fatalf("V1 GET after V2 resource patch: want %q, got %q", res2ID, v1resp.ResourceID)
	}
	if v1resp.AccessMask != 1 {
		t.Fatalf("V1 GET after V2 resource patch: access_mask changed, want 1 got %d", v1resp.AccessMask)
	}
}
