package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
