package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
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
