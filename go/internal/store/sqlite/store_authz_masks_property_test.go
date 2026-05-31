package sqlite

import (
	"context"
	"testing"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

// TestMaterialized_masksMatchGroundTruth exercises all six authz mutation
// methods and after each one verifies that every (user, resource) pair in the
// domain has the same effective mask in user_resource_masks (materialized path)
// as PermissionMasksForUserResource+CombineMasks (ground-truth query path).
//
// This is the primary acceptance criterion for T04.
func TestMaterialized_masksMatchGroundTruth(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domID, Title: "d"}); err != nil {
		t.Fatal(err)
	}

	uid1 := uuid.NewString()
	uid2 := uuid.NewString()
	gid1 := uuid.NewString()
	gid2 := uuid.NewString()
	rid1 := uuid.NewString()
	rid2 := uuid.NewString()
	pid1 := uuid.NewString() // resource rid1, mask 0x1
	pid2 := uuid.NewString() // resource rid1, mask 0x2
	pid3 := uuid.NewString() // resource rid2, mask 0x4

	for _, u := range []*store.User{
		{ID: uid1, DomainID: domID, Title: "u1"},
		{ID: uid2, DomainID: domID, Title: "u2"},
	} {
		if err := s.UserCreate(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []*store.Group{
		{ID: gid1, DomainID: domID, Title: "g1"},
		{ID: gid2, DomainID: domID, Title: "g2"},
	} {
		if err := s.GroupCreate(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []*store.Resource{
		{ID: rid1, DomainID: domID, Title: "r1"},
		{ID: rid2, DomainID: domID, Title: "r2"},
	} {
		if err := s.ResourceCreate(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []*store.Permission{
		{ID: pid1, DomainID: domID, Title: "p1", ResourceID: rid1, AccessMask: 0x1},
		{ID: pid2, DomainID: domID, Title: "p2", ResourceID: rid1, AccessMask: 0x2},
		{ID: pid3, DomainID: domID, Title: "p3", ResourceID: rid2, AccessMask: 0x4},
	} {
		if err := s.PermissionCreate(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	users := []string{uid1, uid2}
	resources := []string{rid1, rid2}

	assertConsistent := func(label string) {
		t.Helper()
		for _, uid := range users {
			for _, rid := range resources {
				got, err := s.EffectiveMask(ctx, domID, uid, rid)
				if err != nil {
					t.Fatalf("%s: EffectiveMask(%s, %s): %v", label, uid, rid, err)
				}
				masks, err := s.PermissionMasksForUserResource(ctx, domID, uid, rid)
				if err != nil {
					t.Fatalf("%s: PermissionMasksForUserResource(%s, %s): %v", label, uid, rid, err)
				}
				want := access.CombineMasks(masks)
				if got != want {
					t.Errorf("%s: EffectiveMask(%s, %s) = %#x, want %#x (ground truth)", label, uid, rid, got, want)
				}
			}
		}
	}

	assertConsistent("initial (no grants)")

	// GrantUserPermission: uid1 gets pid1 (rid1, 0x1)
	if err := s.GrantUserPermission(ctx, domID, uid1, pid1); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after GrantUserPermission uid1+pid1")

	// GrantUserPermission: uid1 gets pid2 (rid1, 0x2) → combined 0x3
	if err := s.GrantUserPermission(ctx, domID, uid1, pid2); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after GrantUserPermission uid1+pid2")

	// AddUserToGroup: uid2 → gid1
	if err := s.AddUserToGroup(ctx, domID, uid2, gid1); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after AddUserToGroup uid2+gid1 (no group permissions yet)")

	// GrantGroupPermission: gid1 gets pid3 (rid2, 0x4) → uid2 gets mask 0x4 on rid2
	if err := s.GrantGroupPermission(ctx, domID, gid1, pid3); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after GrantGroupPermission gid1+pid3")

	// AddUserToGroup: uid1 → gid1 too → uid1 should now also have 0x4 on rid2
	if err := s.AddUserToGroup(ctx, domID, uid1, gid1); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after AddUserToGroup uid1+gid1")

	// GrantGroupPermission: gid2 gets pid1 (rid1, 0x1); no members yet → no change
	if err := s.GrantGroupPermission(ctx, domID, gid2, pid1); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after GrantGroupPermission gid2+pid1 (no members)")

	// AddUserToGroup: uid2 → gid2 → uid2 gains mask 0x1 on rid1 via gid2
	if err := s.AddUserToGroup(ctx, domID, uid2, gid2); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after AddUserToGroup uid2+gid2")

	// RevokeGroupPermission: gid1 loses pid3 → uid1 and uid2 lose 0x4 on rid2
	if err := s.RevokeGroupPermission(ctx, domID, gid1, pid3); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after RevokeGroupPermission gid1+pid3")

	// RemoveUserFromGroup: uid1 leaves gid1
	if err := s.RemoveUserFromGroup(ctx, domID, uid1, gid1); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after RemoveUserFromGroup uid1+gid1")

	// RevokeUserPermission: uid1 loses pid1
	if err := s.RevokeUserPermission(ctx, domID, uid1, pid1); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after RevokeUserPermission uid1+pid1")

	// RevokeUserPermission: uid1 loses pid2 → uid1 should have 0 on rid1
	if err := s.RevokeUserPermission(ctx, domID, uid1, pid2); err != nil {
		t.Fatal(err)
	}
	assertConsistent("after RevokeUserPermission uid1+pid2 (uid1 rid1 should be 0)")

	// Verify uid1 has no entry in user_resource_masks for rid1 (mask=0 → row deleted)
	m, err := s.EffectiveMask(ctx, domID, uid1, rid1)
	if err != nil {
		t.Fatal(err)
	}
	if m != 0 {
		t.Errorf("expected mask 0 for uid1/rid1, got %#x", m)
	}
}

// TestMaterialized_reconcile verifies that ReconcileUserResourceMasks produces
// the same result as write-through after wiping the materialized table.
func TestMaterialized_reconcile(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domID := uuid.NewString()
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domID, Title: "p", ResourceID: rid, AccessMask: 0x7}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domID, gid, pid); err != nil {
		t.Fatal(err)
	}

	wantMask, err := s.EffectiveMask(ctx, domID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if wantMask == 0 {
		t.Fatal("expected non-zero mask before wiping table")
	}

	// Wipe the materialized table to simulate drift / fresh DB.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM user_resource_masks`); err != nil {
		t.Fatal(err)
	}
	zeroMask, err := s.EffectiveMask(ctx, domID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if zeroMask != 0 {
		t.Fatalf("expected 0 after wipe, got %#x", zeroMask)
	}

	// Reconcile should restore the correct mask.
	if err := s.ReconcileUserResourceMasks(ctx); err != nil {
		t.Fatal(err)
	}
	gotMask, err := s.EffectiveMask(ctx, domID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if gotMask != wantMask {
		t.Errorf("after reconcile: mask = %#x, want %#x", gotMask, wantMask)
	}
}

// TestMaterialized_permissionPatchInvalidatesCache verifies that
// PermissionPatch correctly refreshes user_resource_masks when a
// permission's access_mask or resource_id is changed.
func TestMaterialized_permissionPatchInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domID := uuid.NewString()
	uid := uuid.NewString()
	rid1 := uuid.NewString()
	rid2 := uuid.NewString()
	pid := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []*store.Resource{
		{ID: rid1, DomainID: domID, Title: "r1"},
		{ID: rid2, DomainID: domID, Title: "r2"},
	} {
		if err := s.ResourceCreate(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domID, Title: "p", ResourceID: rid1, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domID, uid, pid); err != nil {
		t.Fatal(err)
	}

	// Precondition: mask on rid1 is 0x1, mask on rid2 is 0.
	m1, err := s.EffectiveMask(ctx, domID, uid, rid1)
	if err != nil {
		t.Fatal(err)
	}
	if m1 != 0x1 {
		t.Fatalf("pre-patch: EffectiveMask rid1 = %#x, want 0x1", m1)
	}

	// Patch: change access_mask from 0x1 to 0x3.
	newMask := uint64(0x3)
	if _, err := s.PermissionPatch(ctx, domID, pid, store.PermissionPatchParams{AccessMask: &newMask}); err != nil {
		t.Fatal(err)
	}
	m1After, err := s.EffectiveMask(ctx, domID, uid, rid1)
	if err != nil {
		t.Fatal(err)
	}
	if m1After != 0x3 {
		t.Errorf("after mask patch: EffectiveMask rid1 = %#x, want 0x3", m1After)
	}

	// Patch: reassign permission to rid2 (mask stays 0x3).
	if _, err := s.PermissionPatch(ctx, domID, pid, store.PermissionPatchParams{ResourceID: &rid2}); err != nil {
		t.Fatal(err)
	}
	// Old resource should now have mask 0 (cache row deleted).
	m1Old, err := s.EffectiveMask(ctx, domID, uid, rid1)
	if err != nil {
		t.Fatal(err)
	}
	if m1Old != 0 {
		t.Errorf("after resource reassign: EffectiveMask rid1 = %#x, want 0", m1Old)
	}
	// New resource should reflect the mask.
	m2New, err := s.EffectiveMask(ctx, domID, uid, rid2)
	if err != nil {
		t.Fatal(err)
	}
	if m2New != 0x3 {
		t.Errorf("after resource reassign: EffectiveMask rid2 = %#x, want 0x3", m2New)
	}
}
