package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

// ---------------------------------------------------------------------------
// 1. Multi-domain isolation
// ---------------------------------------------------------------------------

func TestIntegration_multiDomainIsolation(t *testing.T) {
	ts, _ := newTestAPI(t)

	domA := seedDomain(t, ts, "DomainA")
	domB := seedDomain(t, ts, "DomainB")
	baseA := domainBase(ts, domA)
	baseB := domainBase(ts, domB)

	userA := seedUser(t, ts, domA, "SharedName")
	userB := seedUser(t, ts, domB, "SharedName")
	groupA := seedGroup(t, ts, domA, "SharedGroup")
	_ = seedGroup(t, ts, domB, "SharedGroup")
	resA := seedResource(t, ts, domA, "SharedRes")
	_ = seedResource(t, ts, domB, "SharedRes")
	_ = seedAccessType(t, ts, domA, "read", "0x1")
	_ = seedAccessType(t, ts, domB, "read", "0x1")
	permA := seedPermission(t, ts, domA, "can-read", resA, "0x1")

	t.Run("list_isolation_users", func(t *testing.T) {
		var envA listResponse[store.User]
		if err := json.Unmarshal(mustGet(t, baseA+"/users", http.StatusOK), &envA); err != nil {
			t.Fatal(err)
		}
		if envA.Meta.Total != 1 {
			t.Fatalf("domain A users total: want 1, got %d", envA.Meta.Total)
		}
		if len(envA.Data) != 1 || envA.Data[0].ID != userA {
			t.Fatalf("domain A users: %+v", envA.Data)
		}

		var envB listResponse[store.User]
		if err := json.Unmarshal(mustGet(t, baseB+"/users", http.StatusOK), &envB); err != nil {
			t.Fatal(err)
		}
		if envB.Meta.Total != 1 {
			t.Fatalf("domain B users total: want 1, got %d", envB.Meta.Total)
		}
		if len(envB.Data) != 1 || envB.Data[0].ID != userB {
			t.Fatalf("domain B users: %+v", envB.Data)
		}
	})

	t.Run("get_isolation", func(t *testing.T) {
		mustGet(t, baseA+"/users/"+userA, http.StatusOK)
		mustGet(t, baseB+"/users/"+userA, http.StatusNotFound)
		mustGet(t, baseA+"/users/"+userB, http.StatusNotFound)
		mustGet(t, baseB+"/users/"+userB, http.StatusOK)
	})

	t.Run("authz_isolation", func(t *testing.T) {
		addMembership(t, ts, domA, userA, groupA)
		grantGroupPerm(t, ts, domA, groupA, permA)

		checkURL := func(domID, uID, rID string) string {
			return domainBase(ts, domID) + fmt.Sprintf("/authz/check?user_id=%s&resource_id=%s&access_bit=0x1", uID, rID)
		}

		var outA struct{ Allowed bool `json:"allowed"` }
		if err := json.Unmarshal(mustGet(t, checkURL(domA, userA, resA), http.StatusOK), &outA); err != nil {
			t.Fatal(err)
		}
		if !outA.Allowed {
			t.Fatal("domain A user should be allowed in domain A")
		}

		var outB struct{ Allowed bool `json:"allowed"` }
		if err := json.Unmarshal(mustGet(t, checkURL(domB, userA, resA), http.StatusOK), &outB); err != nil {
			t.Fatal(err)
		}
		if outB.Allowed {
			t.Fatal("domain A user should NOT be allowed in domain B")
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Concurrent write safety
// ---------------------------------------------------------------------------

func TestIntegration_concurrentWrites(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "concurrent")
	base := domainBase(ts, domID)

	const n = 20
	errs := make(chan error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			title := fmt.Sprintf("user-%d", i)

			b, err := doRequestErr(http.MethodPost, base+"/users",
				fmt.Sprintf(`{"title":%q}`, title), http.StatusCreated)
			if err != nil {
				errs <- err
				return
			}
			var created struct{ ID string }
			if err := json.Unmarshal(b, &created); err != nil {
				errs <- fmt.Errorf("create user %d unmarshal: %w", i, err)
				return
			}

			if _, err := doRequestErr(http.MethodPatch, base+"/users/"+created.ID,
				fmt.Sprintf(`{"title":%q}`, title+"-patched"), http.StatusOK); err != nil {
				errs <- err
				return
			}
			if _, err := doRequestErr(http.MethodDelete, base+"/users/"+created.ID,
				"", http.StatusNoContent); err != nil {
				errs <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	var env listResponse[store.User]
	if err := json.Unmarshal(mustGet(t, base+"/users", http.StatusOK), &env); err != nil {
		t.Fatal(err)
	}
	if env.Meta.Total != 0 {
		t.Fatalf("want 0 users after concurrent delete-all, got %d", env.Meta.Total)
	}
}

func TestIntegration_concurrentMembership(t *testing.T) {
	ts, _ := newTestAPI(t)
	domID := seedDomain(t, ts, "membership")
	base := domainBase(ts, domID)
	gid := seedGroup(t, ts, domID, "group")

	const n = 15
	uids := make([]string, n)
	for i := 0; i < n; i++ {
		uids[i] = seedUser(t, ts, domID, fmt.Sprintf("u-%d", i))
	}

	addErrs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			_, err := doRequestErr(http.MethodPost,
				base+"/users/"+uid+"/groups/"+gid, "", http.StatusNoContent)
			if err != nil {
				addErrs <- err
			}
		}(uids[i])
	}
	wg.Wait()
	close(addErrs)
	for err := range addErrs {
		t.Error(err)
	}

	delErrs := make(chan error, n)
	var wg2 sync.WaitGroup
	for i := 0; i < n; i++ {
		wg2.Add(1)
		go func(uid string) {
			defer wg2.Done()
			_, err := doRequestErr(http.MethodDelete,
				base+"/users/"+uid+"/groups/"+gid, "", http.StatusNoContent)
			if err != nil {
				delErrs <- err
			}
		}(uids[i])
	}
	wg2.Wait()
	close(delErrs)
	for err := range delErrs {
		t.Error(err)
	}

	var env listResponse[store.User]
	if err := json.Unmarshal(mustGet(t, base+"/users", http.StatusOK), &env); err != nil {
		t.Fatal(err)
	}
	if int(env.Meta.Total) != n {
		t.Fatalf("users should still exist (%d), got total=%d", n, env.Meta.Total)
	}
}

// ---------------------------------------------------------------------------
// 3. Referential integrity challenges
// ---------------------------------------------------------------------------

func TestIntegration_referentialIntegrity(t *testing.T) {
	ts, _ := newTestAPI(t)

	t.Run("domain_with_users", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-dom")
		_ = seedUser(t, ts, did, "u")
		mustDeleteReq(t, domainBase(ts, did), http.StatusBadRequest)
		mustGet(t, ts.URL+"/api/v1/domains/"+did, http.StatusOK)
	})

	t.Run("group_with_child_group", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-group-parent")
		parent := seedGroup(t, ts, did, "parent")
		mustPostJSON(t, domainBase(ts, did)+"/groups",
			fmt.Sprintf(`{"title":"child","parent_group_id":%q}`, parent), http.StatusCreated)

		mustDeleteReq(t, domainBase(ts, did)+"/groups/"+parent, http.StatusBadRequest)
		mustGet(t, domainBase(ts, did)+"/groups/"+parent, http.StatusOK)
	})

	t.Run("group_with_members", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-group-members")
		gid := seedGroup(t, ts, did, "g")
		uid := seedUser(t, ts, did, "u")
		addMembership(t, ts, did, uid, gid)

		mustDeleteReq(t, domainBase(ts, did)+"/groups/"+gid, http.StatusBadRequest)
		mustGet(t, domainBase(ts, did)+"/groups/"+gid, http.StatusOK)
	})

	t.Run("user_with_membership", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-user-member")
		gid := seedGroup(t, ts, did, "g")
		uid := seedUser(t, ts, did, "u")
		addMembership(t, ts, did, uid, gid)

		mustDeleteReq(t, domainBase(ts, did)+"/users/"+uid, http.StatusBadRequest)
		mustGet(t, domainBase(ts, did)+"/users/"+uid, http.StatusOK)
	})

	t.Run("resource_with_permissions", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-res-perm")
		rid := seedResource(t, ts, did, "r")
		_ = seedPermission(t, ts, did, "p", rid, "0x1")

		mustDeleteReq(t, domainBase(ts, did)+"/resources/"+rid, http.StatusBadRequest)
		mustGet(t, domainBase(ts, did)+"/resources/"+rid, http.StatusOK)
	})

	t.Run("permission_with_user_grants", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-perm-user")
		rid := seedResource(t, ts, did, "r")
		pid := seedPermission(t, ts, did, "p", rid, "0x1")
		uid := seedUser(t, ts, did, "u")
		grantUserPerm(t, ts, did, uid, pid)

		mustDeleteReq(t, domainBase(ts, did)+"/permissions/"+pid, http.StatusBadRequest)
		mustGet(t, domainBase(ts, did)+"/permissions/"+pid, http.StatusOK)
	})

	t.Run("permission_with_group_grants", func(t *testing.T) {
		did := seedDomain(t, ts, "ref-perm-group")
		rid := seedResource(t, ts, did, "r")
		pid := seedPermission(t, ts, did, "p", rid, "0x1")
		gid := seedGroup(t, ts, did, "g")
		grantGroupPerm(t, ts, did, gid, pid)

		mustDeleteReq(t, domainBase(ts, did)+"/permissions/"+pid, http.StatusBadRequest)
		mustGet(t, domainBase(ts, did)+"/permissions/"+pid, http.StatusOK)
	})
}

// ---------------------------------------------------------------------------
// 4. Idempotency / conflict checks
// ---------------------------------------------------------------------------

func TestIntegration_idempotencyConflicts(t *testing.T) {
	ts, _ := newTestAPI(t)

	t.Run("double_membership", func(t *testing.T) {
		did := seedDomain(t, ts, "idem-mem")
		uid := seedUser(t, ts, did, "u")
		gid := seedGroup(t, ts, did, "g")
		addMembership(t, ts, did, uid, gid)

		mustPostEmpty(t, domainBase(ts, did)+"/users/"+uid+"/groups/"+gid, http.StatusConflict)
	})

	t.Run("double_user_grant", func(t *testing.T) {
		did := seedDomain(t, ts, "idem-ugrant")
		uid := seedUser(t, ts, did, "u")
		rid := seedResource(t, ts, did, "r")
		pid := seedPermission(t, ts, did, "p", rid, "0x1")
		grantUserPerm(t, ts, did, uid, pid)

		mustPostEmpty(t, domainBase(ts, did)+"/users/"+uid+"/permissions/"+pid, http.StatusConflict)
	})

	t.Run("double_group_grant", func(t *testing.T) {
		did := seedDomain(t, ts, "idem-ggrant")
		gid := seedGroup(t, ts, did, "g")
		rid := seedResource(t, ts, did, "r")
		pid := seedPermission(t, ts, did, "p", rid, "0x1")
		grantGroupPerm(t, ts, did, gid, pid)

		mustPostEmpty(t, domainBase(ts, did)+"/groups/"+gid+"/permissions/"+pid, http.StatusConflict)
	})

	t.Run("duplicate_access_type_bit", func(t *testing.T) {
		did := seedDomain(t, ts, "idem-at")
		_ = seedAccessType(t, ts, did, "read", "0x1")

		mustPostJSON(t, domainBase(ts, did)+"/access-types",
			`{"title":"write","bit":"0x1"}`, http.StatusConflict)
	})
}

// ---------------------------------------------------------------------------
// 5. Permission mask arithmetic
// ---------------------------------------------------------------------------

func TestIntegration_permissionMaskArithmetic(t *testing.T) {
	ts, _ := newTestAPI(t)
	did := seedDomain(t, ts, "masks")
	base := domainBase(ts, did)

	_ = seedAccessType(t, ts, did, "bit0", "0x1")
	_ = seedAccessType(t, ts, did, "bit1", "0x2")
	_ = seedAccessType(t, ts, did, "bit2", "0x4")
	_ = seedAccessType(t, ts, did, "bit3", "0x8")

	r1 := seedResource(t, ts, did, "R1")
	r2 := seedResource(t, ts, did, "R2")

	p1 := seedPermission(t, ts, did, "P1", r1, "0x3")
	p2 := seedPermission(t, ts, did, "P2", r1, "0xC")
	_ = seedPermission(t, ts, did, "P3", r2, "0x5")

	uid := seedUser(t, ts, did, "U")
	gid := seedGroup(t, ts, did, "G")
	addMembership(t, ts, did, uid, gid)

	grantUserPerm(t, ts, did, uid, p1)
	grantGroupPerm(t, ts, did, gid, p2)

	t.Run("masks_endpoint", func(t *testing.T) {
		url := fmt.Sprintf("%s/authz/masks?user_id=%s&resource_id=%s", base, uid, r1)
		var out struct{ Masks []uint64 `json:"masks"` }
		if err := json.Unmarshal(mustGet(t, url, http.StatusOK), &out); err != nil {
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
	})

	t.Run("check_direct_bit1", func(t *testing.T) {
		assertAuthzCheck(t, base, uid, r1, "0x1", true)
	})
	t.Run("check_direct_bit2", func(t *testing.T) {
		assertAuthzCheck(t, base, uid, r1, "0x2", true)
	})
	t.Run("check_group_bit4", func(t *testing.T) {
		assertAuthzCheck(t, base, uid, r1, "0x4", true)
	})
	t.Run("check_group_bit8", func(t *testing.T) {
		assertAuthzCheck(t, base, uid, r1, "0x8", true)
	})
	t.Run("check_no_grant_on_r2", func(t *testing.T) {
		assertAuthzCheck(t, base, uid, r2, "0x1", false)
	})

	// Must run last: revokes p1, which invalidates earlier check_direct_bit* assertions.
	t.Run("revoke_and_recheck", func(t *testing.T) {
		revokeUserPerm(t, ts, did, uid, p1)

		assertAuthzCheck(t, base, uid, r1, "0x1", false)
		assertAuthzCheck(t, base, uid, r1, "0x2", false)
		assertAuthzCheck(t, base, uid, r1, "0x4", true)
		assertAuthzCheck(t, base, uid, r1, "0x8", true)
	})
}

func assertAuthzCheck(t *testing.T, base, userID, resourceID, bit string, wantAllowed bool) {
	t.Helper()
	url := fmt.Sprintf("%s/authz/check?user_id=%s&resource_id=%s&access_bit=%s",
		base, userID, resourceID, bit)
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.Unmarshal(mustGet(t, url, http.StatusOK), &out); err != nil {
		t.Fatal(err)
	}
	if out.Allowed != wantAllowed {
		t.Fatalf("authz/check(user=%s, res=%s, bit=%s): want allowed=%v, got %v",
			userID, resourceID, bit, wantAllowed, out.Allowed)
	}
}

// ---------------------------------------------------------------------------
// 6. Pagination + filtering combined — deferred until T35 lands.
// TODO(T35): Add filter + pagination tests when filtering is implemented.
// ---------------------------------------------------------------------------
