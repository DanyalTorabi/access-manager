package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

func TestAPI_v2_userAuthzResources_returnsTitles(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read"]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantUserPerm(t, ts, domID, userID, perm.ID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var env listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("want 1 resource, got %d", len(env.Data))
	}
	if env.Data[0].ResourceID != resID {
		t.Fatalf("resource_id: want %q, got %q", resID, env.Data[0].ResourceID)
	}
	if len(env.Data[0].Permissions) != 1 || env.Data[0].Permissions[0] != "read" {
		t.Fatalf("permissions: want [read], got %v", env.Data[0].Permissions)
	}
}

func TestAPI_v2_groupAuthzResources_returnsTitles(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "write")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["write"]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantGroupPerm(t, ts, domID, grpID, perm.ID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/groups/"+grpID+"/authz/resources", http.StatusOK)
	var env listResponse[groupAuthzResourceV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("want 1 resource, got %d", len(env.Data))
	}
	if len(env.Data[0].Permissions) != 1 || env.Data[0].Permissions[0] != "write" {
		t.Fatalf("permissions: want [write], got %v", env.Data[0].Permissions)
	}
}

func TestAPI_v2_resourceAuthzUsers_returnsTitles(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read"]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantUserPerm(t, ts, domID, userID, perm.ID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/users", http.StatusOK)
	var env listResponse[resourceAuthzUserV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("want 1 user, got %d", len(env.Data))
	}
	if len(env.Data[0].Permissions) != 1 || env.Data[0].Permissions[0] != "read" {
		t.Fatalf("permissions: want [read], got %v", env.Data[0].Permissions)
	}
}

func TestAPI_v2_resourceAuthzGroups_returnsTitles(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "delete")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["delete"]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantGroupPerm(t, ts, domID, grpID, perm.ID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/groups", http.StatusOK)
	var env listResponse[resourceAuthzGroupV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("want 1 group, got %d", len(env.Data))
	}
	if len(env.Data[0].Permissions) != 1 || env.Data[0].Permissions[0] != "delete" {
		t.Fatalf("permissions: want [delete], got %v", env.Data[0].Permissions)
	}
}

func TestAPI_v2_userResourcePermissions_returnsEffectiveTitles(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	body := fmt.Sprintf(`{"title":"p","resource_id":%q,"permissions":["read","write"]}`, resID)
	var perm permissionResponseV2
	if err := json.Unmarshal(mustPostJSON201(t, domainBaseV2(ts, domID)+"/permissions", body), &perm); err != nil {
		t.Fatal(err)
	}
	grantUserPerm(t, ts, domID, userID, perm.ID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/resources/"+resID+"/permissions", http.StatusOK)
	var resp userResourcePermissionsV2Response
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Permissions) != 2 || resp.Permissions[0] != "read" || resp.Permissions[1] != "write" {
		t.Fatalf("effective permissions: want [read write], got %v", resp.Permissions)
	}
}

func TestAPI_v2_userResourcePermissions_noPermissions_returnsEmpty(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/resources/"+resID+"/permissions", http.StatusOK)
	var resp userResourcePermissionsV2Response
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Permissions) != 0 {
		t.Fatalf("want empty permissions, got %v", resp.Permissions)
	}
}

// TestAPI_v2_authz_v1RegressionCheck verifies that v1 authz endpoints still
// return numeric masks while v2 returns titles for the same underlying data.
func TestAPI_v2_authz_v1RegressionCheck(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")
	// Register via v1 with explicit bit.
	seedAccessType(t, ts, domID, "read", "1")

	permID := seedPermission(t, ts, domID, "p", resID, "1")
	grantUserPerm(t, ts, domID, userID, permID)

	// v1 authz resources: effective_mask is numeric string.
	b := mustGet(t, domainBase(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var v1env listResponse[userAuthzResourceDTO]
	if err := json.Unmarshal(b, &v1env); err != nil {
		t.Fatal(err)
	}
	if len(v1env.Data) != 1 || v1env.Data[0].EffectiveMask != "1" {
		t.Fatalf("v1 authz resources: want effective_mask=1, got %+v", v1env.Data)
	}

	// v2 authz resources: same resource, permissions as title array.
	b2 := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var v2env listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(b2, &v2env); err != nil {
		t.Fatal(err)
	}
	if len(v2env.Data) != 1 || len(v2env.Data[0].Permissions) != 1 || v2env.Data[0].Permissions[0] != "read" {
		t.Fatalf("v2 authz resources: want [read], got %+v", v2env.Data)
	}
}

// --- Unsupported query params (T70) ---

func TestAPI_v2_userAuthzResources_unsupportedQueryParams(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources?search=foo", http.StatusBadRequest)
	var out map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

func TestAPI_v2_groupAuthzResources_unsupportedQueryParams(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	grpID := seedGroup(t, ts, domID, "g")

	b := mustGet(t, domainBaseV2(ts, domID)+"/groups/"+grpID+"/authz/resources?search=foo", http.StatusBadRequest)
	var out map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

func TestAPI_v2_resourceAuthzUsers_unsupportedQueryParams(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")

	b := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/users?search=foo", http.StatusBadRequest)
	var out map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

func TestAPI_v2_resourceAuthzGroups_unsupportedQueryParams(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")

	b := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/groups?search=foo", http.StatusBadRequest)
	var out map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "only limit and offset are supported" {
		t.Fatalf("unexpected error message: %q", out["error"])
	}
}

// --- NotFound tests (T70) ---

func TestAPI_v2_userAuthzResources_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	uid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}

	mustGet(t, ts.URL+"/api/v2/domains/"+uuid.NewString()+"/users/"+uid+"/authz/resources", http.StatusNotFound)
	mustGet(t, ts.URL+"/api/v2/domains/"+domainID+"/users/"+uuid.NewString()+"/authz/resources", http.StatusNotFound)
}

func TestAPI_v2_groupAuthzResources_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	gid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	mustGet(t, ts.URL+"/api/v2/domains/"+uuid.NewString()+"/groups/"+gid+"/authz/resources", http.StatusNotFound)
	mustGet(t, ts.URL+"/api/v2/domains/"+domainID+"/groups/"+uuid.NewString()+"/authz/resources", http.StatusNotFound)
}

func TestAPI_v2_resourceAuthzUsers_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	rid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	mustGet(t, ts.URL+"/api/v2/domains/"+uuid.NewString()+"/resources/"+rid+"/authz/users", http.StatusNotFound)
	mustGet(t, ts.URL+"/api/v2/domains/"+domainID+"/resources/"+uuid.NewString()+"/authz/users", http.StatusNotFound)
}

func TestAPI_v2_resourceAuthzGroups_notFound(t *testing.T) {
	ts, st := newTestAPI(t)
	ctx := context.Background()
	domainID := uuid.NewString()
	rid := uuid.NewString()
	if err := st.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	mustGet(t, ts.URL+"/api/v2/domains/"+uuid.NewString()+"/resources/"+rid+"/authz/groups", http.StatusNotFound)
	mustGet(t, ts.URL+"/api/v2/domains/"+domainID+"/resources/"+uuid.NewString()+"/authz/groups", http.StatusNotFound)
}

// --- Integration tests for V2 list endpoints (T70) ---

// TestAPI_v2_userAuthzResources_viaGroupMembership verifies that a user who
// has access only through group membership sees the correct titles in V2.
func TestAPI_v2_userAuthzResources_viaGroupMembership(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	permID := seedPermissionV2(t, ts, domID, "p", resID, []string{"read", "write"})
	addMembership(t, ts, domID, userID, grpID)
	grantGroupPerm(t, ts, domID, grpID, permID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var env listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 || len(env.Data) != 1 {
		t.Fatalf("want 1 resource, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
	if env.Data[0].ResourceID != resID {
		t.Fatalf("resource_id: want %q got %q", resID, env.Data[0].ResourceID)
	}
	if len(env.Data[0].Permissions) != 2 {
		t.Fatalf("via group: want 2 permission titles, got %v", env.Data[0].Permissions)
	}
}

// TestAPI_v2_userAuthzResources_pagination verifies that V2 list results can
// be paginated with ?limit= and ?offset=.
func TestAPI_v2_userAuthzResources_pagination(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	seedAccessTypeV2(t, ts, domID, "read")

	// Create 3 resources and grant a permission on each.
	var resIDs []string
	for i := 0; i < 3; i++ {
		rid := seedResource(t, ts, domID, fmt.Sprintf("res%d", i))
		resIDs = append(resIDs, rid)
		permID := seedPermissionV2(t, ts, domID, fmt.Sprintf("p%d", i), rid, []string{"read"})
		grantUserPerm(t, ts, domID, userID, permID)
	}

	// Full list.
	bAll := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var envAll listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(bAll, &envAll); err != nil {
		t.Fatal(err)
	}
	if envAll.Meta.Total != 3 {
		t.Fatalf("total: want 3, got %d", envAll.Meta.Total)
	}

	// Page: limit=1 offset=1.
	bPage := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources?limit=1&offset=1", http.StatusOK)
	var envPage listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(bPage, &envPage); err != nil {
		t.Fatal(err)
	}
	if envPage.Meta.Total != 3 || len(envPage.Data) != 1 {
		t.Fatalf("page: want total=3 len=1, got total=%d len=%d", envPage.Meta.Total, len(envPage.Data))
	}

	sortedIDs := append([]string(nil), resIDs...)
	sort.Strings(sortedIDs)
	if envPage.Data[0].ResourceID != sortedIDs[1] {
		t.Fatalf("page[1]: want %s, got %s", sortedIDs[1], envPage.Data[0].ResourceID)
	}
}

// TestAPI_v2_userAuthzResources_unionMasks verifies that when a user has both
// a direct grant and a group grant on the same resource, the V2 list returns
// the union of all titles (EffectiveMask).
func TestAPI_v2_userAuthzResources_unionMasks(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	// Direct user grant: read only.
	directPermID := seedPermissionV2(t, ts, domID, "direct", resID, []string{"read"})
	grantUserPerm(t, ts, domID, userID, directPermID)

	// Group grant: write only; user is a member of the group.
	groupPermID := seedPermissionV2(t, ts, domID, "group-perm", resID, []string{"write"})
	addMembership(t, ts, domID, userID, grpID)
	grantGroupPerm(t, ts, domID, grpID, groupPermID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var env listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 || len(env.Data) != 1 {
		t.Fatalf("want 1 resource, got total=%d len=%d", env.Meta.Total, len(env.Data))
	}
	// EffectiveMask = read | write → both titles present.
	if len(env.Data[0].Permissions) != 2 {
		t.Fatalf("union: want 2 titles (read+write), got %v", env.Data[0].Permissions)
	}
}

// TestAPI_v2_resourceAuthzUsers_integration creates two users with different
// effective masks on the same resource and verifies V2 returns correct titles.
func TestAPI_v2_resourceAuthzUsers_integration(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	user1 := seedUser(t, ts, domID, "u1")
	user2 := seedUser(t, ts, domID, "u2")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	perm1 := seedPermissionV2(t, ts, domID, "p1", resID, []string{"read"})
	perm2 := seedPermissionV2(t, ts, domID, "p2", resID, []string{"write"})
	grantUserPerm(t, ts, domID, user1, perm1)
	grantUserPerm(t, ts, domID, user2, perm2)

	b := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/users", http.StatusOK)
	var env listResponse[resourceAuthzUserV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2, got %d", env.Meta.Total)
	}
	gotPerms := map[string][]string{}
	for _, it := range env.Data {
		gotPerms[it.UserID] = it.Permissions
	}
	if len(gotPerms[user1]) != 1 || gotPerms[user1][0] != "read" {
		t.Fatalf("user1 perms: want [read], got %v", gotPerms[user1])
	}
	if len(gotPerms[user2]) != 1 || gotPerms[user2][0] != "write" {
		t.Fatalf("user2 perms: want [write], got %v", gotPerms[user2])
	}
}

// TestAPI_v2_resourceAuthzGroups_integration creates two groups with grants on
// the same resource and verifies V2 returns correct titles for each.
func TestAPI_v2_resourceAuthzGroups_integration(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	grp1 := seedGroup(t, ts, domID, "g1")
	grp2 := seedGroup(t, ts, domID, "g2")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	perm1 := seedPermissionV2(t, ts, domID, "p1", resID, []string{"read"})
	perm2 := seedPermissionV2(t, ts, domID, "p2", resID, []string{"read", "write"})
	grantGroupPerm(t, ts, domID, grp1, perm1)
	grantGroupPerm(t, ts, domID, grp2, perm2)

	b := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/groups", http.StatusOK)
	var env listResponse[resourceAuthzGroupV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2, got %d", env.Meta.Total)
	}
	gotPerms := map[string][]string{}
	for _, it := range env.Data {
		gotPerms[it.GroupID] = it.Permissions
	}
	if len(gotPerms[grp1]) != 1 || gotPerms[grp1][0] != "read" {
		t.Fatalf("grp1 perms: want [read], got %v", gotPerms[grp1])
	}
	if len(gotPerms[grp2]) != 2 {
		t.Fatalf("grp2 perms: want [read write], got %v", gotPerms[grp2])
	}
}

// TestAPI_v2_groupAuthzResources_integration verifies that a group's authz
// resource list covers multiple resources with correct titles.
func TestAPI_v2_groupAuthzResources_integration(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	grpID := seedGroup(t, ts, domID, "g")
	resA := seedResource(t, ts, domID, "resA")
	resB := seedResource(t, ts, domID, "resB")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")

	permA := seedPermissionV2(t, ts, domID, "pA", resA, []string{"read"})
	permB := seedPermissionV2(t, ts, domID, "pB", resB, []string{"read", "write"})
	grantGroupPerm(t, ts, domID, grpID, permA)
	grantGroupPerm(t, ts, domID, grpID, permB)

	b := mustGet(t, domainBaseV2(ts, domID)+"/groups/"+grpID+"/authz/resources", http.StatusOK)
	var env listResponse[groupAuthzResourceV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2, got %d", env.Meta.Total)
	}
	gotPerms := map[string][]string{}
	for _, it := range env.Data {
		gotPerms[it.ResourceID] = it.Permissions
	}
	if len(gotPerms[resA]) != 1 || gotPerms[resA][0] != "read" {
		t.Fatalf("resA perms: want [read], got %v", gotPerms[resA])
	}
	if len(gotPerms[resB]) != 2 {
		t.Fatalf("resB perms: want [read write], got %v", gotPerms[resB])
	}
}

// TestAPI_v2_userResourcePermissions_viaGroupAndDirect verifies that the
// effective permissions endpoint unions direct and group-inherited grants.
func TestAPI_v2_userResourcePermissions_viaGroupAndDirect(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")
	seedAccessTypeV2(t, ts, domID, "write")
	seedAccessTypeV2(t, ts, domID, "delete")

	// Direct: read.
	directPerm := seedPermissionV2(t, ts, domID, "direct", resID, []string{"read"})
	grantUserPerm(t, ts, domID, userID, directPerm)
	// Group: write+delete; user is member.
	groupPerm := seedPermissionV2(t, ts, domID, "group-perm", resID, []string{"write", "delete"})
	addMembership(t, ts, domID, userID, grpID)
	grantGroupPerm(t, ts, domID, grpID, groupPerm)

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/resources/"+resID+"/permissions", http.StatusOK)
	var resp userResourcePermissionsV2Response
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Permissions) != 3 {
		t.Fatalf("effective: want 3 titles (read+write+delete), got %v", resp.Permissions)
	}
}

// TestAPI_v2_resourceAuthzUsers_noDomainLeakage verifies that users from a
// different domain do not appear in V2 resource authz user results.
func TestAPI_v2_resourceAuthzUsers_noDomainLeakage(t *testing.T) {
	ts, _ := newTestAPI(t)
	domA := seedDomain(t, ts, "domA")
	domB := seedDomain(t, ts, "domB")

	userA := seedUser(t, ts, domA, "userA")
	userB := seedUser(t, ts, domB, "userB")
	resA := seedResource(t, ts, domA, "resA")
	resB := seedResource(t, ts, domB, "resB")

	seedAccessTypeV2(t, ts, domA, "read")
	seedAccessTypeV2(t, ts, domB, "read")

	permA := seedPermissionV2(t, ts, domA, "pA", resA, []string{"read"})
	permB := seedPermissionV2(t, ts, domB, "pB", resB, []string{"read"})
	grantUserPerm(t, ts, domA, userA, permA)
	grantUserPerm(t, ts, domB, userB, permB)

	// Domain A resource sees only userA.
	b := mustGet(t, domainBaseV2(ts, domA)+"/resources/"+resA+"/authz/users", http.StatusOK)
	var env listResponse[resourceAuthzUserV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 {
		t.Fatalf("domain leakage: want 1 user in domA, got %d", env.Meta.Total)
	}
	if env.Data[0].UserID != userA {
		t.Fatalf("domain leakage: want userA, got %q", env.Data[0].UserID)
	}
}

// TestAPI_v2_userAuthzResources_allPermissions verifies that a user with all
// available access types on a resource sees all titles in the V2 response.
func TestAPI_v2_userAuthzResources_allPermissions(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")

	titles := []string{"read", "write", "delete"}
	for _, title := range titles {
		seedAccessTypeV2(t, ts, domID, title)
	}
	permID := seedPermissionV2(t, ts, domID, "all", resID, titles)
	grantUserPerm(t, ts, domID, userID, permID)

	b := mustGet(t, domainBaseV2(ts, domID)+"/users/"+userID+"/authz/resources", http.StatusOK)
	var env listResponse[userAuthzResourceV2]
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 1 || len(env.Data) != 1 {
		t.Fatalf("want 1 resource, got total=%d", env.Meta.Total)
	}
	if len(env.Data[0].Permissions) != 3 {
		t.Fatalf("all permissions: want 3, got %v", env.Data[0].Permissions)
	}
}

// --- Cross-version compatibility: authzCheck and authzMasks (T70) ---

// TestAPI_v2_authzCheck_grantedViaUserPermission verifies that a permission
// created via the V2 API (title-based) is accessible via the V1 authzCheck
// endpoint. This is a new scenario not covered by V1's existing
// TestAPI_authzCheck_viaGroup_integration (which tests via group membership).
func TestAPI_v2_authzCheck_grantedViaUserPermission(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")

	// Register access type and create permission via V2 (title-based).
	seedAccessTypeV2(t, ts, domID, "read")
	permID := seedPermissionV2(t, ts, domID, "p", resID, []string{"read"})
	grantUserPerm(t, ts, domID, userID, permID)

	// V1 authzCheck with access_bit=0x1 (bit 1 = "read") must return allowed=true.
	q := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=%s&resource_id=%s&access_bit=0x1",
		ts.URL, domID, userID, resID)
	b := mustGet(t, q, http.StatusOK)
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Allowed {
		t.Fatalf("V1 authzCheck after V2 grant: expected allowed=true, got %+v", out)
	}
}

// TestAPI_v2_authzMasks_userAndGroup verifies that the V1 authzMasks endpoint
// returns the correct union of direct user grants and group-inherited grants
// when permissions were created via the V2 API.
func TestAPI_v2_authzMasks_userAndGroup(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")  // bit=1
	seedAccessTypeV2(t, ts, domID, "write") // bit=2

	// Direct grant: read (bit 1).
	directPerm := seedPermissionV2(t, ts, domID, "direct", resID, []string{"read"})
	grantUserPerm(t, ts, domID, userID, directPerm)
	// Group grant: write (bit 2); user is member.
	groupPerm := seedPermissionV2(t, ts, domID, "group-perm", resID, []string{"write"})
	addMembership(t, ts, domID, userID, grpID)
	grantGroupPerm(t, ts, domID, grpID, groupPerm)

	q := fmt.Sprintf("%s/api/v1/domains/%s/authz/masks?user_id=%s&resource_id=%s",
		ts.URL, domID, userID, resID)
	b := mustGet(t, q, http.StatusOK)
	var out struct {
		Masks []uint64 `json:"masks"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	// Expect at least two mask entries (one for each permission).
	if len(out.Masks) < 2 {
		t.Fatalf("authzMasks user+group: want ≥2 mask entries, got %v", out.Masks)
	}
}

// TestAPI_v2_authzCheck_emptyDomain verifies that authzCheck on a domain with
// no access types at all returns allowed=false (mask is zero; no bit can match).
func TestAPI_v2_authzCheck_emptyDomain(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")

	q := fmt.Sprintf("%s/api/v1/domains/%s/authz/check?user_id=%s&resource_id=%s&access_bit=0x1",
		ts.URL, domID, userID, resID)
	b := mustGet(t, q, http.StatusOK)
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Allowed {
		t.Fatalf("empty domain: expected allowed=false, got true")
	}
}

// --- Duplicate grant tests with V2-created permissions (T70) ---

// TestAPI_v2_grantUserPermission_duplicate grants a permission created via V2
// to the same user twice and expects a 409 Conflict on the second attempt.
func TestAPI_v2_grantUserPermission_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	userID := seedUser(t, ts, domID, "u")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	permID := seedPermissionV2(t, ts, domID, "p", resID, []string{"read"})
	grantURL := domainBase(ts, domID) + "/users/" + userID + "/permissions/" + permID

	req1, _ := http.NewRequest(http.MethodPost, grantURL, nil)
	res1, err := testClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first grant: want 204, got %d", res1.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodPost, grantURL, nil)
	res2, err := testClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate grant: want 409, got %d", res2.StatusCode)
	}
}

// TestAPI_v2_grantGroupPermission_duplicate grants a permission created via V2
// to the same group twice and expects a 409 Conflict on the second attempt.
func TestAPI_v2_grantGroupPermission_duplicate(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	grpID := seedGroup(t, ts, domID, "g")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	permID := seedPermissionV2(t, ts, domID, "p", resID, []string{"read"})
	grantURL := domainBase(ts, domID) + "/groups/" + grpID + "/permissions/" + permID

	req1, _ := http.NewRequest(http.MethodPost, grantURL, nil)
	res1, err := testClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	_ = res1.Body.Close()
	if res1.StatusCode != http.StatusNoContent {
		t.Fatalf("first grant: want 204, got %d", res1.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodPost, grantURL, nil)
	res2, err := testClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate grant: want 409, got %d", res2.StatusCode)
	}
}

// TestAPI_v2_resourceAuthzUsers_pagination verifies that the V2
// resourceAuthzUsers endpoint paginates correctly.
func TestAPI_v2_resourceAuthzUsers_pagination(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "d")
	resID := seedResource(t, ts, domID, "res")
	seedAccessTypeV2(t, ts, domID, "read")

	for i := 0; i < 3; i++ {
		uid := seedUser(t, ts, domID, fmt.Sprintf("u%d", i))
		permID := seedPermissionV2(t, ts, domID, fmt.Sprintf("p%d", i), resID, []string{"read"})
		grantUserPerm(t, ts, domID, uid, permID)
	}

	bAll := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/users", http.StatusOK)
	var envAll listResponse[resourceAuthzUserV2]
	if err := json.Unmarshal(bAll, &envAll); err != nil {
		t.Fatal(err)
	}
	if envAll.Meta.Total != 3 {
		t.Fatalf("total: want 3, got %d", envAll.Meta.Total)
	}

	bPage := mustGet(t, domainBaseV2(ts, domID)+"/resources/"+resID+"/authz/users?limit=2&offset=0", http.StatusOK)
	var envPage listResponse[resourceAuthzUserV2]
	if err := json.Unmarshal(bPage, &envPage); err != nil {
		t.Fatal(err)
	}
	if envPage.Meta.Total != 3 || len(envPage.Data) != 2 {
		t.Fatalf("page: want total=3 len=2, got total=%d len=%d", envPage.Meta.Total, len(envPage.Data))
	}
}

// seedPermissionV2 creates a permission via the V2 API and returns its ID.
// It uses the title-based permissions array.
func seedPermissionV2(t *testing.T, ts *httptest.Server, domID, title, resID string, perms []string) string {
	t.Helper()
	permJSON := "["
	for i, p := range perms {
		if i > 0 {
			permJSON += ","
		}
		permJSON += fmt.Sprintf("%q", p)
	}
	permJSON += "]"
	body := fmt.Sprintf(`{"title":%q,"resource_id":%q,"permissions":%s}`, title, resID, permJSON)
	b := mustPostJSON(t, domainBaseV2(ts, domID)+"/permissions", body, http.StatusCreated)
	var out struct{ ID string }
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.ID
}
