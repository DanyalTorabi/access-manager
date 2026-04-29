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
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
	pid := seedPermission(t, c, did, "p", rid, "0x1")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)

	grantUserPerm(t, c, did, uid, pid)
	cleanupRevokeUserPerm(t, c, did, uid, pid)

	assertAuthzCheck(t, c, did, uid, rid, "0x1", true)
}

func TestAuthz_groupInherited(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-group")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	gid := seedGroup(t, c, did, "g")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
	pid := seedPermission(t, c, did, "p", rid, "0x2")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)

	addMembership(t, c, did, uid, gid)
	cleanupRevokeMembership(t, c, did, uid, gid)
	grantGroupPerm(t, c, did, gid, pid)
	cleanupRevokeGroupPerm(t, c, did, gid, pid)

	assertAuthzCheck(t, c, did, uid, rid, "0x2", true)
}

func TestAuthz_noPermission(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-none")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
	pid := seedPermission(t, c, did, "p", rid, "0x1")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)

	assertAuthzCheck(t, c, did, uid, rid, "0x1", false)
}

func TestAuthz_multipleMasks(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-multi")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	gid := seedGroup(t, c, did, "g")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)

	p1 := seedPermission(t, c, did, "p1", rid, "0x3")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+p1)
	p2 := seedPermission(t, c, did, "p2", rid, "0xC")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+p2)

	grantUserPerm(t, c, did, uid, p1)
	cleanupRevokeUserPerm(t, c, did, uid, p1)
	addMembership(t, c, did, uid, gid)
	cleanupRevokeMembership(t, c, did, uid, gid)
	grantGroupPerm(t, c, did, gid, p2)
	cleanupRevokeGroupPerm(t, c, did, gid, p2)

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
}

func TestAuthz_revokeAndRecheck(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-revoke")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
	pid := seedPermission(t, c, did, "p", rid, "0x1")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)

	grantUserPerm(t, c, did, uid, pid)
	cleanupRevokeUserPerm(t, c, did, uid, pid)
	assertAuthzCheck(t, c, did, uid, rid, "0x1", true)

	revokeUserPerm(t, c, did, uid, pid)
	assertAuthzCheck(t, c, did, uid, rid, "0x1", false)
}

func TestAuthz_userResourcesList(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-user-resources")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)

	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	gid := seedGroup(t, c, did, "g")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)

	addMembership(t, c, did, uid, gid)
	cleanupRevokeMembership(t, c, did, uid, gid)

	ridA := seedResource(t, c, did, "a")
	ridB := seedResource(t, c, did, "b")
	ridC := seedResource(t, c, did, "c")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+ridA)
	cleanupDelete(t, c, domainBase(did)+"/resources/"+ridB)
	cleanupDelete(t, c, domainBase(did)+"/resources/"+ridC)

	pUserA := seedPermission(t, c, did, "pUserA", ridA, "0x1")
	pGroupA := seedPermission(t, c, did, "pGroupA", ridA, "0x4")
	pGroupB := seedPermission(t, c, did, "pGroupB", ridB, "0x2")
	pUserC1 := seedPermission(t, c, did, "pUserC1", ridC, "0x8")
	pUserC2 := seedPermission(t, c, did, "pUserC2", ridC, "0x10")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pUserA)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pGroupA)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pGroupB)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pUserC1)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pUserC2)

	grantUserPerm(t, c, did, uid, pUserA)
	grantGroupPerm(t, c, did, gid, pGroupA)
	grantGroupPerm(t, c, did, gid, pGroupB)
	grantUserPerm(t, c, did, uid, pUserC1)
	grantUserPerm(t, c, did, uid, pUserC2)
	cleanupRevokeUserPerm(t, c, did, uid, pUserA)
	cleanupRevokeGroupPerm(t, c, did, gid, pGroupA)
	cleanupRevokeGroupPerm(t, c, did, gid, pGroupB)
	cleanupRevokeUserPerm(t, c, did, uid, pUserC1)
	cleanupRevokeUserPerm(t, c, did, uid, pUserC2)

	type authzResource struct {
		ResourceID    string `json:"resource_id"`
		EffectiveMask string `json:"effective_mask"`
	}

	base := fmt.Sprintf("%s/users/%s/authz/resources", domainBase(did), uid)
	env := mustList(t, c, base+"?offset=0&limit=10")
	if env.Meta.Total != 3 {
		t.Fatalf("total: want 3, got %d", env.Meta.Total)
	}
	var items []authzResource
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items len: want 3, got %d", len(items))
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.ResourceID] = it.EffectiveMask
	}
	if got[ridA] != "5" {
		t.Fatalf("ridA mask: want 5, got %q", got[ridA])
	}
	if got[ridB] != "2" {
		t.Fatalf("ridB mask: want 2, got %q", got[ridB])
	}
	if got[ridC] != "24" {
		t.Fatalf("ridC mask: want 24, got %q", got[ridC])
	}

	page := mustList(t, c, base+"?offset=1&limit=1")
	if page.Meta.Total != 3 {
		t.Fatalf("page total: want 3, got %d", page.Meta.Total)
	}
	var pageItems []authzResource
	if err := json.Unmarshal(page.Data, &pageItems); err != nil {
		t.Fatal(err)
	}
	if len(pageItems) != 1 {
		t.Fatalf("page len: want 1, got %d", len(pageItems))
	}
	orderedIDs := []string{ridA, ridB, ridC}
	sort.Strings(orderedIDs)
	if pageItems[0].ResourceID != orderedIDs[1] {
		t.Fatalf("page resource: want %s, got %s", orderedIDs[1], pageItems[0].ResourceID)
	}
}

// TestAuthz_nestedGroupScaffold is a scaffold for nested-group inheritance (T2).
// Current behaviour: grant to parent, user in child -> NOT inherited (flat
// group model). When T2 lands, flip wantAllowed to true.
func TestAuthz_nestedGroupScaffold(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-nested")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)
	uid := seedUser(t, c, did, "u")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uid)
	parentGID := seedGroup(t, c, did, "parent")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+parentGID)
	childGID := seedGroupWithParent(t, c, did, "child", parentGID)
	cleanupDelete(t, c, domainBase(did)+"/groups/"+childGID)
	cleanupUnlinkParent(t, c, did, childGID)
	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)
	pid := seedPermission(t, c, did, "p", rid, "0x1")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pid)

	addMembership(t, c, did, uid, childGID)
	cleanupRevokeMembership(t, c, did, uid, childGID)
	grantGroupPerm(t, c, did, parentGID, pid)
	cleanupRevokeGroupPerm(t, c, did, parentGID, pid)

	// TODO(T2): When nested-group inheritance lands, change to true.
	assertAuthzCheck(t, c, did, uid, rid, "0x1", false)
}

func TestAuthz_groupResourcesList(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-group-resources")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)

	gid := seedGroup(t, c, did, "g")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)

	ridA := seedResource(t, c, did, "a")
	ridB := seedResource(t, c, did, "b")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+ridA)
	cleanupDelete(t, c, domainBase(did)+"/resources/"+ridB)

	// Two permissions on ridA (OR = 0x1|0x4 = 5), one on ridB (0x2).
	pA1 := seedPermission(t, c, did, "pA1", ridA, "0x1")
	pA2 := seedPermission(t, c, did, "pA2", ridA, "0x4")
	pB := seedPermission(t, c, did, "pB", ridB, "0x2")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pA1)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pA2)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pB)

	grantGroupPerm(t, c, did, gid, pA1)
	grantGroupPerm(t, c, did, gid, pA2)
	grantGroupPerm(t, c, did, gid, pB)
	cleanupRevokeGroupPerm(t, c, did, gid, pA1)
	cleanupRevokeGroupPerm(t, c, did, gid, pA2)
	cleanupRevokeGroupPerm(t, c, did, gid, pB)

	type groupAuthzResource struct {
		ResourceID string `json:"resource_id"`
		Mask       string `json:"mask"`
	}

	base := fmt.Sprintf("%s/groups/%s/authz/resources", domainBase(did), gid)
	env := mustList(t, c, base+"?offset=0&limit=10")
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2, got %d", env.Meta.Total)
	}
	var items []groupAuthzResource
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items len: want 2, got %d", len(items))
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.ResourceID] = it.Mask
	}
	if got[ridA] != "5" {
		t.Fatalf("ridA mask: want 5, got %q", got[ridA])
	}
	if got[ridB] != "2" {
		t.Fatalf("ridB mask: want 2, got %q", got[ridB])
	}

	page := mustList(t, c, base+"?offset=1&limit=1")
	if page.Meta.Total != 2 {
		t.Fatalf("page total: want 2, got %d", page.Meta.Total)
	}
	var pageItems []groupAuthzResource
	if err := json.Unmarshal(page.Data, &pageItems); err != nil {
		t.Fatal(err)
	}
	if len(pageItems) != 1 {
		t.Fatalf("page len: want 1, got %d", len(pageItems))
	}
	orderedIDs := []string{ridA, ridB}
	sort.Strings(orderedIDs)
	if pageItems[0].ResourceID != orderedIDs[1] {
		t.Fatalf("page resource: want %s, got %s", orderedIDs[1], pageItems[0].ResourceID)
	}
}

func TestAuthz_resourceUsersList(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-resource-users")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)

	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)

	uA := seedUser(t, c, did, "uA")
	uB := seedUser(t, c, did, "uB")
	uC := seedUser(t, c, did, "uC")
	cleanupDelete(t, c, domainBase(did)+"/users/"+uA)
	cleanupDelete(t, c, domainBase(did)+"/users/"+uB)
	cleanupDelete(t, c, domainBase(did)+"/users/"+uC)

	gid := seedGroup(t, c, did, "g")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gid)
	addMembership(t, c, did, uA, gid)
	addMembership(t, c, did, uB, gid)
	cleanupRevokeMembership(t, c, did, uA, gid)
	cleanupRevokeMembership(t, c, did, uB, gid)

	pDirect := seedPermission(t, c, did, "pDirect", rid, "0x1")
	pGroup := seedPermission(t, c, did, "pGroup", rid, "0x4")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pDirect)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pGroup)

	grantUserPerm(t, c, did, uA, pDirect)
	grantGroupPerm(t, c, did, gid, pGroup)
	cleanupRevokeUserPerm(t, c, did, uA, pDirect)
	cleanupRevokeGroupPerm(t, c, did, gid, pGroup)

	type authzUser struct {
		UserID        string `json:"user_id"`
		EffectiveMask string `json:"effective_mask"`
	}

	base := fmt.Sprintf("%s/resources/%s/authz/users", domainBase(did), rid)
	env := mustList(t, c, base+"?offset=0&limit=10")
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2 (uA + uB; uC has no access), got %d", env.Meta.Total)
	}
	var items []authzUser
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items len: want 2, got %d", len(items))
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.UserID] = it.EffectiveMask
	}
	if got[uA] != "5" {
		t.Fatalf("uA mask: want 5 (0x1 direct | 0x4 group), got %q", got[uA])
	}
	if got[uB] != "4" {
		t.Fatalf("uB mask: want 4 (group only), got %q", got[uB])
	}
	if _, ok := got[uC]; ok {
		t.Fatalf("uC must not appear (no grants)")
	}

	// Pagination: ordered by user_id ASC.
	page := mustList(t, c, base+"?offset=1&limit=1")
	if page.Meta.Total != 2 {
		t.Fatalf("page total: want 2, got %d", page.Meta.Total)
	}
	var pageItems []authzUser
	if err := json.Unmarshal(page.Data, &pageItems); err != nil {
		t.Fatal(err)
	}
	if len(pageItems) != 1 {
		t.Fatalf("page len: want 1, got %d", len(pageItems))
	}
	orderedIDs := []string{uA, uB}
	sort.Strings(orderedIDs)
	if pageItems[0].UserID != orderedIDs[1] {
		t.Fatalf("page user: want %s, got %s", orderedIDs[1], pageItems[0].UserID)
	}
}

func TestAuthz_resourceGroupsList(t *testing.T) {
	c := httpClient()
	did := seedDomain(t, c, "authz-resource-groups")
	cleanupDelete(t, c, apiBase()+"/domains/"+did)

	rid := seedResource(t, c, did, "r")
	cleanupDelete(t, c, domainBase(did)+"/resources/"+rid)

	gA := seedGroup(t, c, did, "gA")
	gB := seedGroup(t, c, did, "gB")
	gX := seedGroup(t, c, did, "gX")
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gA)
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gB)
	cleanupDelete(t, c, domainBase(did)+"/groups/"+gX)

	// Two grants for gA on rid (OR = 0x1|0x4 = 5), one grant for gB (0x2).
	pA1 := seedPermission(t, c, did, "pA1", rid, "0x1")
	pA2 := seedPermission(t, c, did, "pA2", rid, "0x4")
	pB := seedPermission(t, c, did, "pB", rid, "0x2")
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pA1)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pA2)
	cleanupDelete(t, c, domainBase(did)+"/permissions/"+pB)

	grantGroupPerm(t, c, did, gA, pA1)
	grantGroupPerm(t, c, did, gA, pA2)
	grantGroupPerm(t, c, did, gB, pB)
	cleanupRevokeGroupPerm(t, c, did, gA, pA1)
	cleanupRevokeGroupPerm(t, c, did, gA, pA2)
	cleanupRevokeGroupPerm(t, c, did, gB, pB)

	type authzGroup struct {
		GroupID string `json:"group_id"`
		Mask    string `json:"mask"`
	}

	base := fmt.Sprintf("%s/resources/%s/authz/groups", domainBase(did), rid)
	env := mustList(t, c, base+"?offset=0&limit=10")
	if env.Meta.Total != 2 {
		t.Fatalf("total: want 2 (gA + gB; gX has no grants), got %d", env.Meta.Total)
	}
	var items []authzGroup
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items len: want 2, got %d", len(items))
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.GroupID] = it.Mask
	}
	if got[gA] != "5" {
		t.Fatalf("gA mask: want 5 (0x1|0x4), got %q", got[gA])
	}
	if got[gB] != "2" {
		t.Fatalf("gB mask: want 2, got %q", got[gB])
	}
	if _, ok := got[gX]; ok {
		t.Fatalf("gX must not appear (no grants)")
	}

	// Pagination: ordered by group_id ASC.
	page := mustList(t, c, base+"?offset=1&limit=1")
	if page.Meta.Total != 2 {
		t.Fatalf("page total: want 2, got %d", page.Meta.Total)
	}
	var pageItems []authzGroup
	if err := json.Unmarshal(page.Data, &pageItems); err != nil {
		t.Fatal(err)
	}
	if len(pageItems) != 1 {
		t.Fatalf("page len: want 1, got %d", len(pageItems))
	}
	orderedIDs := []string{gA, gB}
	sort.Strings(orderedIDs)
	if pageItems[0].GroupID != orderedIDs[1] {
		t.Fatalf("page group: want %s, got %s", orderedIDs[1], pageItems[0].GroupID)
	}
}
