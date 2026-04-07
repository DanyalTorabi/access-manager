//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Authz challenge scenarios
// ---------------------------------------------------------------------------

func TestAuthz_directPermission(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-direct")
	uid := seedUser(t, c, did, "u")
	rid := seedResource(t, c, did, "r")
	pid := seedPermission(t, c, did, "p", rid, "0x1")

	grantUserPerm(t, c, did, uid, pid)

	assertAuthzCheck(t, c, did, uid, rid, "0x1", true)

	revokeUserPerm(t, c, did, uid, pid)
	mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestAuthz_groupInherited(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-group")
	uid := seedUser(t, c, did, "u")
	gid := seedGroup(t, c, did, "g")
	rid := seedResource(t, c, did, "r")
	pid := seedPermission(t, c, did, "p", rid, "0x2")

	addMembership(t, c, did, uid, gid)
	grantGroupPerm(t, c, did, gid, pid)

	assertAuthzCheck(t, c, did, uid, rid, "0x2", true)

	revokeGroupPerm(t, c, did, gid, pid)
	removeMembership(t, c, did, uid, gid)
	mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/groups/"+gid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestAuthz_noPermission(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-none")
	uid := seedUser(t, c, did, "u")
	rid := seedResource(t, c, did, "r")
	pid := seedPermission(t, c, did, "p", rid, "0x1")

	assertAuthzCheck(t, c, did, uid, rid, "0x1", false)

	mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestAuthz_multipleMasks(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-multi")
	uid := seedUser(t, c, did, "u")
	gid := seedGroup(t, c, did, "g")
	rid := seedResource(t, c, did, "r")

	p1 := seedPermission(t, c, did, "p1", rid, "0x3")
	p2 := seedPermission(t, c, did, "p2", rid, "0xC")

	grantUserPerm(t, c, did, uid, p1)
	addMembership(t, c, did, uid, gid)
	grantGroupPerm(t, c, did, gid, p2)

	// /authz/masks
	url := fmt.Sprintf("%s/authz/masks?user_id=%s&resource_id=%s", domainBase(did), uid, rid)
	var out authzMasksResp
	if err := json.Unmarshal(mustGET(t, c, url, http.StatusOK), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Masks) != 2 {
		t.Fatalf("want 2 masks, got %d: %v", len(out.Masks), out.Masks)
	}
	got := make([]uint64, len(out.Masks))
	copy(got, out.Masks)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if got[0] != 3 || got[1] != 12 {
		t.Fatalf("want masks [3, 12], got %v", got)
	}

	// Each bit individually
	assertAuthzCheck(t, c, did, uid, rid, "0x1", true)
	assertAuthzCheck(t, c, did, uid, rid, "0x2", true)
	assertAuthzCheck(t, c, did, uid, rid, "0x4", true)
	assertAuthzCheck(t, c, did, uid, rid, "0x8", true)

	revokeGroupPerm(t, c, did, gid, p2)
	revokeUserPerm(t, c, did, uid, p1)
	removeMembership(t, c, did, uid, gid)
	mustDELETE(t, c, domainBase(did)+"/permissions/"+p2, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/permissions/"+p1, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/groups/"+gid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

func TestAuthz_revokeAndRecheck(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-revoke")
	uid := seedUser(t, c, did, "u")
	rid := seedResource(t, c, did, "r")
	pid := seedPermission(t, c, did, "p", rid, "0x1")

	grantUserPerm(t, c, did, uid, pid)
	assertAuthzCheck(t, c, did, uid, rid, "0x1", true)

	revokeUserPerm(t, c, did, uid, pid)
	assertAuthzCheck(t, c, did, uid, rid, "0x1", false)

	mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}

// TestAuthz_nestedGroupScaffold is a scaffold for nested-group inheritance (T2).
// Current behaviour: grant to parent, user in child -> NOT inherited (flat
// group model). When T2 lands, flip wantAllowed to true.
func TestAuthz_nestedGroupScaffold(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-nested")
	uid := seedUser(t, c, did, "u")
	parentGID := seedGroup(t, c, did, "parent")
	childGID := seedGroupWithParent(t, c, did, "child", parentGID)
	rid := seedResource(t, c, did, "r")
	pid := seedPermission(t, c, did, "p", rid, "0x1")

	addMembership(t, c, did, uid, childGID)
	grantGroupPerm(t, c, did, parentGID, pid)

	// TODO(T2): When nested-group inheritance lands, change to true.
	assertAuthzCheck(t, c, did, uid, rid, "0x1", false)

	revokeGroupPerm(t, c, did, parentGID, pid)
	removeMembership(t, c, did, uid, childGID)
	mustDELETE(t, c, domainBase(did)+"/permissions/"+pid, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/resources/"+rid, http.StatusNoContent)
	mustPATCH(t, c, domainBase(did)+"/groups/"+childGID+"/parent", `{"parent_group_id":null}`, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/groups/"+childGID, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/groups/"+parentGID, http.StatusNoContent)
	mustDELETE(t, c, domainBase(did)+"/users/"+uid, http.StatusNoContent)
	mustDELETE(t, c, apiBase()+"/domains/"+did, http.StatusNoContent)
}
