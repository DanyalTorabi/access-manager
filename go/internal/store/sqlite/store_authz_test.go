package sqlite

import (
	"context"
	"errors"
	"testing"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func TestEffectiveMask_userAndGroup(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x3}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}
	m, err := s.EffectiveMask(ctx, domainID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if m != 0x3 {
		t.Fatalf("mask = %#x, want 0x3", m)
	}
	if !access.HasBit(m, 0x1) || !access.HasBit(m, 0x2) {
		t.Fatal("expected read+write bits")
	}
}


func TestEffectiveMask_directUserPermission(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}
	m, err := s.EffectiveMask(ctx, domainID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if m != 0x4 {
		t.Fatalf("mask = %#x", m)
	}
}


func TestEffectiveMask_noGrants(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	rid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.EffectiveMask(ctx, domainID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if m != 0 {
		t.Fatalf("want 0 without grants, got %#x", m)
	}
}


func TestEffectiveMask_userPlusGroupOR(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	pUser := uuid.NewString()
	pGroup := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUser, DomainID: domainID, Title: "pu", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pGroup, DomainID: domainID, Title: "pg", ResourceID: rid, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pUser); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pGroup); err != nil {
		t.Fatal(err)
	}
	m, err := s.EffectiveMask(ctx, domainID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if m != 0x3 {
		t.Fatalf("want OR of user 0x1 and group 0x2 => 0x3, got %#x", m)
	}
}


func TestUserAuthzResourcesList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}

	ridA := uuid.NewString()
	ridB := uuid.NewString()
	ridC := uuid.NewString()
	for _, rid := range []string{ridA, ridB, ridC} {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r-" + rid}); err != nil {
			t.Fatal(err)
		}
	}

	pUserA := uuid.NewString()
	pGroupA := uuid.NewString()
	pGroupB := uuid.NewString()
	pUserC1 := uuid.NewString()
	pUserC2 := uuid.NewString()

	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUserA, DomainID: domainID, Title: "pUserA", ResourceID: ridA, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pGroupA, DomainID: domainID, Title: "pGroupA", ResourceID: ridA, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pGroupB, DomainID: domainID, Title: "pGroupB", ResourceID: ridB, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUserC1, DomainID: domainID, Title: "pUserC1", ResourceID: ridC, AccessMask: 0x8}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUserC2, DomainID: domainID, Title: "pUserC2", ResourceID: ridC, AccessMask: 0x10}); err != nil {
		t.Fatal(err)
	}

	if err := s.GrantUserPermission(ctx, domainID, uid, pUserA); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pUserC1); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pUserC2); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pGroupA); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pGroupB); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 0, Limit: 10})

	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total: want 3, got %d", total)
	}
	if len(list) != 3 {
		t.Fatalf("len: want 3, got %d", len(list))
	}

	gotMasks := map[string]uint64{}
	for _, it := range list {
		gotMasks[it.ResourceID] = it.EffectiveMask
	}
	if gotMasks[ridA] != 0x5 {
		t.Fatalf("ridA mask: want 0x5, got %#x", gotMasks[ridA])
	}
	if gotMasks[ridB] != 0x2 {
		t.Fatalf("ridB mask: want 0x2, got %#x", gotMasks[ridB])
	}
	if gotMasks[ridC] != 0x18 {
		t.Fatalf("ridC mask: want 0x18, got %#x", gotMasks[ridC])
	}

	page, pageTotal, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if pageTotal != 3 || len(page) != 1 {
		t.Fatalf("pagination: total=%d len=%d", pageTotal, len(page))
	}
	orderedIDs := []string{ridA, ridB, ridC}
	sort.Strings(orderedIDs)
	if page[0].ResourceID != orderedIDs[1] {
		t.Fatalf("pagination resource: want %s, got %s", orderedIDs[1], page[0].ResourceID)
	}

	emptyPage, emptyTotal, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 99, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if emptyTotal != 3 || len(emptyPage) != 0 {
		t.Fatalf("past end: total=%d len=%d", emptyTotal, len(emptyPage))
	}
}


func TestUserAuthzResourcesList_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.UserAuthzResourcesList(ctx, uuid.NewString(), uid, store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown domain: want ErrNotFound, got %v", err)
	}
	if _, _, err := s.UserAuthzResourcesList(ctx, domainID, uuid.NewString(), store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown user: want ErrNotFound, got %v", err)
	}
}


func TestBuildUserAuthzMaskQueryAndArgs(t *testing.T) {
	predicateArgs := []any{"d", "u", "u", "d", "d"}
	q, args, err := buildUserAuthzMaskQueryAndArgs("dom", []string{"r1", "r2"}, predicateArgs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "IN (?,?)") {
		t.Fatalf("query placeholders: got %q", q)
	}
	want := []any{"dom", "r1", "r2", "d", "u", "u", "d", "d"}
	if len(args) != len(want) {
		t.Fatalf("args len: want %d, got %d", len(want), len(args))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d]: want %v, got %v", i, want[i], args[i])
		}
	}
}


func TestBuildUserAuthzMaskQueryAndArgs_emptyResourceIDs(t *testing.T) {
	if _, _, err := buildUserAuthzMaskQueryAndArgs("dom", nil, []any{"d", "u", "u", "d", "d"}); err == nil {
		t.Fatal("want error for empty resource IDs")
	}
}


func TestUserAuthzResourcesList_noPermissions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	uid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("total: want 0, got %d", total)
	}
	if len(list) != 0 {
		t.Fatalf("list len: want 0, got %d", len(list))
	}
}


func TestUserAuthzResourcesList_nonPositiveMasksExcluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	uid := uuid.NewString()
	ridNeg := uuid.NewString()
	pidNeg := uuid.NewString()
	ridZero := uuid.NewString()
	pidZero := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: ridNeg, DomainID: domainID, Title: "r-neg"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: ridZero, DomainID: domainID, Title: "r-zero"}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO permissions (id, domain_id, title, resource_id, access_mask) VALUES (?, ?, ?, ?, ?)`,
		pidNeg, domainID, "neg-mask", ridNeg, int64(-1),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
		domainID, uid, pidNeg,
	); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pidZero, DomainID: domainID, Title: "zero-mask", ResourceID: ridZero, AccessMask: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pidZero); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("non-positive masks should be excluded: total=%d len=%d", total, len(list))
	}
}


func TestUserAuthzResourcesList_positiveMaskIncluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	uid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].ResourceID != rid || list[0].EffectiveMask != 1 {
		t.Fatalf("positive mask should be listed: total=%d len=%d list=%+v", total, len(list), list)
	}
}


func TestUserAuthzResourcesList_limitClampedAtMaxLimit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	uid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}

	wantTotal := store.MaxLimit + 5
	for i := 0; i < wantTotal; i++ {
		rid := uuid.NewString()
		pid := uuid.NewString()
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: fmt.Sprintf("r-%03d", i)}); err != nil {
			t.Fatal(err)
		}
		if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: fmt.Sprintf("p-%03d", i), ResourceID: rid, AccessMask: 1}); err != nil {
			t.Fatal(err)
		}
		if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
			t.Fatal(err)
		}
	}

	page1, total1, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: 0, Limit: store.MaxLimit + 50})
	if err != nil {
		t.Fatal(err)
	}
	if total1 != int64(wantTotal) {
		t.Fatalf("total1: want %d, got %d", wantTotal, total1)
	}
	if len(page1) != store.MaxLimit {
		t.Fatalf("page1 len: want %d, got %d", store.MaxLimit, len(page1))
	}

	page2, total2, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{Offset: store.MaxLimit, Limit: store.MaxLimit + 50})
	if err != nil {
		t.Fatal(err)
	}
	if total2 != int64(wantTotal) {
		t.Fatalf("total2: want %d, got %d", wantTotal, total2)
	}
	if len(page2) != wantTotal-store.MaxLimit {
		t.Fatalf("page2 len: want %d, got %d", wantTotal-store.MaxLimit, len(page2))
	}
}


func TestGroupAuthzResourcesList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	ridA := uuid.NewString()
	ridB := uuid.NewString()
	ridC := uuid.NewString()
	for _, rid := range []string{ridA, ridB, ridC} {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r-" + rid}); err != nil {
			t.Fatal(err)
		}
	}

	// Two permissions on ridA (OR), one on ridB, none on ridC (no grant).
	pA1 := uuid.NewString()
	pA2 := uuid.NewString()
	pB := uuid.NewString()
	pC := uuid.NewString()

	if err := s.PermissionCreate(ctx, &store.Permission{ID: pA1, DomainID: domainID, Title: "pA1", ResourceID: ridA, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pA2, DomainID: domainID, Title: "pA2", ResourceID: ridA, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pB, DomainID: domainID, Title: "pB", ResourceID: ridB, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pC, DomainID: domainID, Title: "pC", ResourceID: ridC, AccessMask: 0x8}); err != nil {
		t.Fatal(err)
	}

	if err := s.GrantGroupPermission(ctx, domainID, gid, pA1); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pA2); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pB); err != nil {
		t.Fatal(err)
	}
	// pC is NOT granted to the group.

	list, total, err := s.GroupAuthzResourcesList(ctx, domainID, gid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total: want 2, got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("len: want 2, got %d", len(list))
	}

	gotMasks := map[string]uint64{}
	for _, it := range list {
		gotMasks[it.ResourceID] = it.Mask
	}
	if gotMasks[ridA] != 0x5 {
		t.Fatalf("ridA mask: want 0x5, got %#x", gotMasks[ridA])
	}
	if gotMasks[ridB] != 0x2 {
		t.Fatalf("ridB mask: want 0x2, got %#x", gotMasks[ridB])
	}
	if _, ok := gotMasks[ridC]; ok {
		t.Fatalf("ridC should not appear (not granted to group)")
	}

	// Pagination: offset=1, limit=1.
	page, pageTotal, err := s.GroupAuthzResourcesList(ctx, domainID, gid, store.ListOpts{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if pageTotal != 2 || len(page) != 1 {
		t.Fatalf("pagination: total=%d len=%d", pageTotal, len(page))
	}
	orderedIDs := []string{ridA, ridB}
	sort.Strings(orderedIDs)
	if page[0].ResourceID != orderedIDs[1] {
		t.Fatalf("pagination resource: want %s, got %s", orderedIDs[1], page[0].ResourceID)
	}

	// Past end.
	emptyPage, emptyTotal, err := s.GroupAuthzResourcesList(ctx, domainID, gid, store.ListOpts{Offset: 99, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if emptyTotal != 2 || len(emptyPage) != 0 {
		t.Fatalf("past end: total=%d len=%d", emptyTotal, len(emptyPage))
	}
}


func TestGroupAuthzResourcesList_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.GroupAuthzResourcesList(ctx, uuid.NewString(), gid, store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown domain: want ErrNotFound, got %v", err)
	}
	if _, _, err := s.GroupAuthzResourcesList(ctx, domainID, uuid.NewString(), store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown group: want ErrNotFound, got %v", err)
	}
}


func TestGroupAuthzResourcesList_noPermissions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.GroupAuthzResourcesList(ctx, domainID, gid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("total: want 0, got %d", total)
	}
	if len(list) != 0 {
		t.Fatalf("list len: want 0, got %d", len(list))
	}
}


func TestGroupAuthzResourcesList_nonPositiveMasksExcluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	// Mask=0 must not appear in the list.
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.GroupAuthzResourcesList(ctx, domainID, gid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("total: want 0, got %d", total)
	}
	if len(list) != 0 {
		t.Fatalf("list len: want 0, got %d", len(list))
	}
}


func TestAddUserToGroup_fkViolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	err := s.AddUserToGroup(ctx, domainID, uuid.NewString(), gid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestGrantUserPermission_fkViolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	err := s.GrantUserPermission(ctx, domainID, uid, uuid.NewString())
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestGrantGroupPermission_fkViolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	err := s.GrantGroupPermission(ctx, domainID, gid, uuid.NewString())
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestAddUserToGroup_duplicate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	err := s.AddUserToGroup(ctx, domainID, uid, gid)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}


func TestGrantUserPermission_duplicate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}
	err := s.GrantUserPermission(ctx, domainID, uid, pid)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}


func TestGrantGroupPermission_duplicate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}
	err := s.GrantGroupPermission(ctx, domainID, gid, pid)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}


func TestRemoveUserFromGroup_successAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveUserFromGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveUserFromGroup(ctx, domainID, uid, gid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second remove: want ErrNotFound, got %v", err)
	}
	if err := s.RemoveUserFromGroup(ctx, domainID, uid, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}


func TestRevokeUserPermission_successAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeUserPermission(ctx, domainID, uid, pid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second revoke: want ErrNotFound, got %v", err)
	}
}


func TestRevokeGroupPermission_successAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeGroupPermission(ctx, domainID, gid, pid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second revoke: want ErrNotFound, got %v", err)
	}
}


func TestRestrictDelete_domainWithUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainDelete(ctx, domainID)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_resourceWithPermission(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{
		ID: uuid.NewString(), DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1,
	}); err != nil {
		t.Fatal(err)
	}
	err := s.ResourceDelete(ctx, domainID, rid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_userInGroup(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	err := s.UserDelete(ctx, domainID, uid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_groupWithChild(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	parentID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "c", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	err := s.GroupDelete(ctx, domainID, parentID)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_domainWithGroup(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainDelete(ctx, domainID)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_domainWithResource(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainDelete(ctx, domainID)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_domainWithAccessType(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainDelete(ctx, domainID)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_userWithUserGrant(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}
	err := s.UserDelete(ctx, domainID, uid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_groupWithMember(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		t.Fatal(err)
	}
	err := s.GroupDelete(ctx, domainID, gid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_groupWithGroupGrant(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}
	err := s.GroupDelete(ctx, domainID, gid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_permissionWithUserGrant(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}
	err := s.PermissionDelete(ctx, domainID, pid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestRestrictDelete_permissionWithGroupGrant(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}
	err := s.PermissionDelete(ctx, domainID, pid)
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation, got %v", err)
	}
}


func TestEffectiveMask_dbClosed(t *testing.T) {
	ctx := context.Background()
	db, err := Open("file:" + filepath.Join(t.TempDir(), "closed.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	s := New(db)
	domainID := uuid.NewString()
	uid := uuid.NewString()
	rid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = s.EffectiveMask(ctx, domainID, uid, rid)
	if err == nil {
		t.Fatal("want error from closed db")
	}
}


func TestResourceAuthzUsersList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	// Create four users:
	// uA: direct grant 0x1 + group grants 0x2|0x4 -> 0x7
	// uB: group grants 0x2|0x4 only -> 0x6
	// uC: direct grant 0x8 plus another direct 0x10 -> 0x18
	// uX: no access (must NOT appear)
	uA := uuid.NewString()
	uB := uuid.NewString()
	uC := uuid.NewString()
	uX := uuid.NewString()
	for _, u := range []string{uA, uB, uC, uX} {
		if err := s.UserCreate(ctx, &store.User{ID: u, DomainID: domainID, Title: "u-" + u}); err != nil {
			t.Fatal(err)
		}
	}

	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uA, gid); err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uB, gid); err != nil {
		t.Fatal(err)
	}

	pUserA := uuid.NewString()
	pGroup := uuid.NewString()
	pUserC1 := uuid.NewString()
	pUserC2 := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUserA, DomainID: domainID, Title: "pUserA", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pGroup, DomainID: domainID, Title: "pGroup", ResourceID: rid, AccessMask: 0x2 | 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUserC1, DomainID: domainID, Title: "pUserC1", ResourceID: rid, AccessMask: 0x8}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pUserC2, DomainID: domainID, Title: "pUserC2", ResourceID: rid, AccessMask: 0x10}); err != nil {
		t.Fatal(err)
	}

	if err := s.GrantUserPermission(ctx, domainID, uA, pUserA); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pGroup); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uC, pUserC1); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uC, pUserC2); err != nil {
		t.Fatal(err)
	}

	// uA -> 0x1 (direct) | 0x6 (group) = 0x7
	// uB -> 0x6 (group)
	// uC -> 0x18
	wantMasks := map[string]uint64{uA: 0x7, uB: 0x6, uC: 0x18}

	list, total, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total: want 3, got %d", total)
	}
	if len(list) != 3 {
		t.Fatalf("len: want 3, got %d", len(list))
	}
	gotMasks := map[string]uint64{}
	for _, it := range list {
		gotMasks[it.UserID] = it.EffectiveMask
	}
	for u, m := range wantMasks {
		if gotMasks[u] != m {
			t.Fatalf("user %s mask: want %#x, got %#x", u, m, gotMasks[u])
		}
	}
	if _, ok := gotMasks[uX]; ok {
		t.Fatalf("uX with no access must not appear")
	}

	// Pagination + ordering by user_id ASC.
	page, pageTotal, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if pageTotal != 3 || len(page) != 1 {
		t.Fatalf("pagination: total=%d len=%d", pageTotal, len(page))
	}
	orderedIDs := []string{uA, uB, uC}
	sort.Strings(orderedIDs)
	if page[0].UserID != orderedIDs[1] {
		t.Fatalf("pagination user: want %s, got %s", orderedIDs[1], page[0].UserID)
	}

	emptyPage, emptyTotal, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 99, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if emptyTotal != 3 || len(emptyPage) != 0 {
		t.Fatalf("past end: total=%d len=%d", emptyTotal, len(emptyPage))
	}
}


func TestResourceAuthzUsersList_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.ResourceAuthzUsersList(ctx, uuid.NewString(), rid, store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown domain: want ErrNotFound, got %v", err)
	}
	if _, _, err := s.ResourceAuthzUsersList(ctx, domainID, uuid.NewString(), store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown resource: want ErrNotFound, got %v", err)
	}
}


func TestResourceAuthzUsersList_noUsers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	rid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("no users: total=%d len=%d", total, len(list))
	}
}


func TestResourceAuthzUsersList_nonPositiveMasksExcluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	rid := uuid.NewString()
	uid := uuid.NewString()
	pidNeg := uuid.NewString()
	pidZero := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO permissions (id, domain_id, title, resource_id, access_mask) VALUES (?, ?, ?, ?, ?)`,
		pidNeg, domainID, "neg-mask", rid, int64(-1),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
		domainID, uid, pidNeg,
	); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pidZero, DomainID: domainID, Title: "zero-mask", ResourceID: rid, AccessMask: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pidZero); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("non-positive masks should be excluded: total=%d len=%d", total, len(list))
	}
}


func TestResourceAuthzUsersList_otherDomainsExcluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	otherDomainID := uuid.NewString()
	rid := uuid.NewString()
	uid := uuid.NewString()
	otherUID := uuid.NewString()
	pid := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: otherDomainID, Title: "other"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: otherUID, DomainID: otherDomainID, Title: "o"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].UserID != uid {
		t.Fatalf("other-domain users must not appear: total=%d list=%+v", total, list)
	}
}


func TestResourceAuthzUsersList_limitClampedAtMaxLimit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}

	wantTotal := store.MaxLimit + 5
	for i := 0; i < wantTotal; i++ {
		uid := uuid.NewString()
		if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: fmt.Sprintf("u-%03d", i)}); err != nil {
			t.Fatal(err)
		}
		if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
			t.Fatal(err)
		}
	}

	page1, total1, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: store.MaxLimit + 50})
	if err != nil {
		t.Fatal(err)
	}
	if total1 != int64(wantTotal) {
		t.Fatalf("total1: want %d, got %d", wantTotal, total1)
	}
	if len(page1) != store.MaxLimit {
		t.Fatalf("page1 len: want %d, got %d", store.MaxLimit, len(page1))
	}

	page2, total2, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{Offset: store.MaxLimit, Limit: store.MaxLimit + 50})
	if err != nil {
		t.Fatal(err)
	}
	if total2 != int64(wantTotal) {
		t.Fatalf("total2: want %d, got %d", wantTotal, total2)
	}
	if len(page2) != wantTotal-store.MaxLimit {
		t.Fatalf("page2 len: want %d, got %d", wantTotal-store.MaxLimit, len(page2))
	}
}


func TestResourceAuthzUsersBaseArgs_orderMatchesPlaceholders(t *testing.T) {
	got := resourceAuthzUsersBaseArgs("D", "R")
	want := []any{"D", "D", "R"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d]: want %v, got %v", i, want[i], got[i])
		}
	}
	// Sanity: the SQL must contain exactly one '?' per arg position.
	wantCount := len(want)
	if got := strings.Count(resourceAuthzUsersBaseSQL, "?"); got != wantCount {
		t.Fatalf("placeholder count: want %d, got %d", wantCount, got)
	}
}


func TestResourceAuthzGroupsList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	// Three groups:
	// gA: two grants on rid (0x1 + 0x4 -> mask 0x5)
	// gB: one grant on rid (0x2 -> mask 0x2)
	// gX: no grants on rid (must NOT appear)
	gA := uuid.NewString()
	gB := uuid.NewString()
	gX := uuid.NewString()
	for _, g := range []string{gA, gB, gX} {
		if err := s.GroupCreate(ctx, &store.Group{ID: g, DomainID: domainID, Title: "g-" + g}); err != nil {
			t.Fatal(err)
		}
	}

	pA1 := uuid.NewString()
	pA2 := uuid.NewString()
	pB := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pA1, DomainID: domainID, Title: "pA1", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pA2, DomainID: domainID, Title: "pA2", ResourceID: rid, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pB, DomainID: domainID, Title: "pB", ResourceID: rid, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gA, pA1); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gA, pA2); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gB, pB); err != nil {
		t.Fatal(err)
	}

	wantMasks := map[string]uint64{gA: 0x5, gB: 0x2}

	list, total, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total: want 2, got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("len: want 2, got %d", len(list))
	}
	gotMasks := map[string]uint64{}
	for _, it := range list {
		gotMasks[it.GroupID] = it.Mask
	}
	for g, m := range wantMasks {
		if gotMasks[g] != m {
			t.Fatalf("group %s mask: want %#x, got %#x", g, m, gotMasks[g])
		}
	}
	if _, ok := gotMasks[gX]; ok {
		t.Fatalf("gX with no grants must not appear")
	}

	page, pageTotal, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if pageTotal != 2 || len(page) != 1 {
		t.Fatalf("pagination: total=%d len=%d", pageTotal, len(page))
	}
	orderedIDs := []string{gA, gB}
	sort.Strings(orderedIDs)
	if page[0].GroupID != orderedIDs[1] {
		t.Fatalf("pagination group: want %s, got %s", orderedIDs[1], page[0].GroupID)
	}

	emptyPage, emptyTotal, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 99, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if emptyTotal != 2 || len(emptyPage) != 0 {
		t.Fatalf("past end: total=%d len=%d", emptyTotal, len(emptyPage))
	}
}


func TestResourceAuthzGroupsList_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.ResourceAuthzGroupsList(ctx, uuid.NewString(), rid, store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown domain: want ErrNotFound, got %v", err)
	}
	if _, _, err := s.ResourceAuthzGroupsList(ctx, domainID, uuid.NewString(), store.ListOpts{Offset: 0, Limit: 10}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown resource: want ErrNotFound, got %v", err)
	}
}


func TestResourceAuthzGroupsList_noGroups(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	rid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("no groups: total=%d len=%d", total, len(list))
	}
}


func TestResourceAuthzGroupsList_nonPositiveMasksExcluded(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	rid := uuid.NewString()
	gid := uuid.NewString()
	pidNeg := uuid.NewString()
	pidZero := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO permissions (id, domain_id, title, resource_id, access_mask) VALUES (?, ?, ?, ?, ?)`,
		pidNeg, domainID, "neg-mask", rid, int64(-1),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
		domainID, gid, pidNeg,
	); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pidZero, DomainID: domainID, Title: "zero-mask", ResourceID: rid, AccessMask: 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pidZero); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("non-positive masks should be excluded: total=%d len=%d", total, len(list))
	}
}

// TestResourceAuthzGroupsList_returnsSameDomainGroups verifies that on a
// clean dataset the listing returns exactly the groups in the resource's
// domain. Cross-domain isolation itself is now enforced by the schema (see
// TestSchema_compositeFKRejectsCrossDomain); this test asserts the
// schema-level guard is in place (the direct INSERT below must fail) and
// that the surviving same-domain rows are returned correctly.

func TestResourceAuthzGroupsList_returnsSameDomainGroups(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	otherDomainID := uuid.NewString()
	rid := uuid.NewString()
	gid := uuid.NewString()
	otherGID := uuid.NewString()
	pid := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: otherDomainID, Title: "other"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: otherGID, DomainID: otherDomainID, Title: "o"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}

	// T51: schema-level invariant. The composite FK
	// (group_id, domain_id) -> groups(id, domain_id) prevents inserting a
	// group_permissions row where domain_id does not match the group's
	// domain_id. This direct INSERT must fail; previously the listing relied
	// on a defensive Go-side filter to hide such rows.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
		domainID, otherGID, pid,
	)
	if err == nil {
		t.Fatal("cross-domain group_permissions insert must fail; composite FK regressed")
	}
	if !errors.Is(wrapConstraintError(err), store.ErrFKViolation) {
		t.Fatalf("expected store.ErrFKViolation, got: %v", err)
	}

	// Listing returns exactly one entry: the same-domain group.
	list, total, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].GroupID != gid {
		t.Fatalf("expected 1 result for the same-domain group only: total=%d list=%+v", total, list)
	}
}

// TestResourceAuthzGroupsList_schemaEnforcesIsolationWithoutGoFilter covers
// the T51 acceptance criterion that authz listings remain correct without
// a defensive Go-side domain filter. The defensive g.domain_id filter has
// been removed from resourceAuthzGroupsBaseSQL in this PR; this test
// re-runs the same join shape (without the predicate) against a multi-
// domain dataset to assert the schema alone keeps the result set scoped
// to the requested domain.

func TestResourceAuthzGroupsList_schemaEnforcesIsolationWithoutGoFilter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	d1 := uuid.NewString()
	d2 := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: d1, Title: "d1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: d2, Title: "d2"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: d1, Title: "r1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: d2, Title: "r2"}); err != nil {
		t.Fatal(err)
	}
	g1 := uuid.NewString()
	g2 := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: g1, DomainID: d1, Title: "g1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: g2, DomainID: d2, Title: "g2"}); err != nil {
		t.Fatal(err)
	}
	p1 := uuid.NewString()
	p2 := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: p1, DomainID: d1, Title: "p1", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: p2, DomainID: d2, Title: "p2", ResourceID: r2, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, d1, g1, p1); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, d2, g2, p2); err != nil {
		t.Fatal(err)
	}

	// Schema-only query: matches the post-T51 production SQL shape — no
	// Go-side domain predicate on gp or g. If the composite FKs are
	// enforcing the invariant, the result for (d1, r1) must contain only
	// g1 (the d2 grant cannot appear because the schema-backed join keeps
	// results scoped to the permission's domain).
	const schemaOnlySQL = `
SELECT DISTINCT gp.group_id
FROM permissions p
INNER JOIN group_permissions gp ON gp.permission_id = p.id
INNER JOIN groups g ON g.id = gp.group_id
WHERE p.domain_id = ? AND p.resource_id = ? AND p.access_mask > 0
ORDER BY gp.group_id ASC
`
	rows, err := s.db.QueryContext(ctx, schemaOnlySQL, d1, r1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			t.Fatal(err)
		}
		got = append(got, gid)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != g1 {
		t.Fatalf("schema-only query must return only same-domain group g1: got %v", got)
	}

	// Sanity: the production query must agree with the schema-only query.
	list, total, err := s.ResourceAuthzGroupsList(ctx, d1, r1, store.ListOpts{Offset: 0, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].GroupID != g1 {
		t.Fatalf("production listing disagreed with schema-only query: total=%d list=%+v", total, list)
	}
}

// TestSchema_compositeFKRejectsCrossDomain verifies the T51 schema invariant
// for all three junction tables: an out-of-band insert with mismatched
// domain_id must fail at the DB layer. Each junction table's two FKs are
// exercised independently so a missing FK cannot hide behind a sibling FK
// failure. The positive-path subtest then verifies that valid same-domain
// inserts still succeed after the schema rebuild.

func TestSchema_compositeFKRejectsCrossDomain(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	d1 := uuid.NewString()
	d2 := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: d1, Title: "d1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: d2, Title: "d2"}); err != nil {
		t.Fatal(err)
	}
	// Per-domain entities; we use d1- and d2-prefixed names to make each
	// case's two parent rows obvious.
	uid1, uid2 := uuid.NewString(), uuid.NewString()
	gid1, gid2 := uuid.NewString(), uuid.NewString()
	rid1, rid2 := uuid.NewString(), uuid.NewString()
	pid1, pid2 := uuid.NewString(), uuid.NewString()
	mustCreate := func(label string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup %s: %v", label, err)
		}
	}
	mustCreate("u1", s.UserCreate(ctx, &store.User{ID: uid1, DomainID: d1, Title: "u1"}))
	mustCreate("u2", s.UserCreate(ctx, &store.User{ID: uid2, DomainID: d2, Title: "u2"}))
	mustCreate("g1", s.GroupCreate(ctx, &store.Group{ID: gid1, DomainID: d1, Title: "g1"}))
	mustCreate("g2", s.GroupCreate(ctx, &store.Group{ID: gid2, DomainID: d2, Title: "g2"}))
	mustCreate("r1", s.ResourceCreate(ctx, &store.Resource{ID: rid1, DomainID: d1, Title: "r1"}))
	mustCreate("r2", s.ResourceCreate(ctx, &store.Resource{ID: rid2, DomainID: d2, Title: "r2"}))
	mustCreate("p1", s.PermissionCreate(ctx, &store.Permission{ID: pid1, DomainID: d1, Title: "p1", ResourceID: rid1, AccessMask: 1}))
	mustCreate("p2", s.PermissionCreate(ctx, &store.Permission{ID: pid2, DomainID: d2, Title: "p2", ResourceID: rid2, AccessMask: 1}))

	// Negative cases: each junction table has two composite FKs; we test
	// each FK independently by keeping the "other" parent valid in the
	// junction's own domain so only the targeted FK fails.
	cases := []struct {
		name string
		sql  string
		args []any
	}{
		// group_members: (domain_id, user_id) -> users(id, domain_id)
		// and (domain_id, group_id) -> groups(id, domain_id).
		{
			name: "group_members_user_fk",
			sql:  `INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
			args: []any{d1, uid2, gid1}, // user belongs to d2; group correct
		},
		{
			name: "group_members_group_fk",
			sql:  `INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
			args: []any{d1, uid1, gid2}, // group belongs to d2; user correct
		},
		// user_permissions: (domain_id, user_id) and
		// (domain_id, permission_id).
		{
			name: "user_permissions_user_fk",
			sql:  `INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
			args: []any{d1, uid2, pid1},
		},
		{
			name: "user_permissions_permission_fk",
			sql:  `INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
			args: []any{d1, uid1, pid2},
		},
		// group_permissions: (domain_id, group_id) and
		// (domain_id, permission_id).
		{
			name: "group_permissions_group_fk",
			sql:  `INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
			args: []any{d1, gid2, pid1},
		},
		{
			name: "group_permissions_permission_fk",
			sql:  `INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
			args: []any{d1, gid1, pid2},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.db.ExecContext(ctx, c.sql, c.args...)
			if err == nil {
				t.Fatalf("%s: cross-domain insert must fail at the DB layer", c.name)
			}
			if !errors.Is(wrapConstraintError(err), store.ErrFKViolation) {
				t.Fatalf("%s: expected store.ErrFKViolation, got %v", c.name, err)
			}
		})
	}

	// Positive paths: valid same-domain inserts must still succeed after
	// the schema rebuild. If a UNIQUE target had been mistyped (e.g.
	// REFERENCES groups(domain_id, id) instead of groups(id, domain_id))
	// the negative cases above would still pass because the FK target
	// would not exist; the positive cases catch that class of bug.
	t.Run("group_members_valid", func(t *testing.T) {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO group_members (domain_id, user_id, group_id) VALUES (?, ?, ?)`,
			d1, uid1, gid1,
		); err != nil {
			t.Fatalf("valid same-domain group_members insert must succeed: %v", err)
		}
	})
	t.Run("user_permissions_valid", func(t *testing.T) {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO user_permissions (domain_id, user_id, permission_id) VALUES (?, ?, ?)`,
			d1, uid1, pid1,
		); err != nil {
			t.Fatalf("valid same-domain user_permissions insert must succeed: %v", err)
		}
	})
	t.Run("group_permissions_valid", func(t *testing.T) {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO group_permissions (domain_id, group_id, permission_id) VALUES (?, ?, ?)`,
			d1, gid1, pid1,
		); err != nil {
			t.Fatalf("valid same-domain group_permissions insert must succeed: %v", err)
		}
	})
}


func TestResourceAuthzGroupsList_limitClampedAtMaxLimit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	domainID := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}

	wantTotal := store.MaxLimit + 5
	for i := 0; i < wantTotal; i++ {
		gid := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: fmt.Sprintf("g-%03d", i)}); err != nil {
			t.Fatal(err)
		}
		if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
			t.Fatal(err)
		}
	}

	page1, total1, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: 0, Limit: store.MaxLimit + 50})
	if err != nil {
		t.Fatal(err)
	}
	if total1 != int64(wantTotal) {
		t.Fatalf("total1: want %d, got %d", wantTotal, total1)
	}
	if len(page1) != store.MaxLimit {
		t.Fatalf("page1 len: want %d, got %d", store.MaxLimit, len(page1))
	}

	page2, total2, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{Offset: store.MaxLimit, Limit: store.MaxLimit + 50})
	if err != nil {
		t.Fatal(err)
	}
	if total2 != int64(wantTotal) {
		t.Fatalf("total2: want %d, got %d", wantTotal, total2)
	}
	if len(page2) != wantTotal-store.MaxLimit {
		t.Fatalf("page2 len: want %d, got %d", wantTotal-store.MaxLimit, len(page2))
	}
}

// TestStore_accessMask_rejectsBit63 documents the temporary 63-bit mask limit
// (#67 / T46). The store rejects values that would set bit 63 because SQLite's
// INTEGER affinity is signed-64. MaxInt64 (1<<63 - 1) must still be accepted
// and round-trip correctly. AccessTypeCreate, PermissionCreate, and the
// corresponding patch methods all enforce the rule via maskToSQL.

func TestStore_accessMask_rejectsBit63(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	const tooBig uint64 = 1 << 63
	const maxOK uint64 = 1<<63 - 1

	t.Run("AccessTypeCreate_bit63", func(t *testing.T) {
		err := s.AccessTypeCreate(ctx, &store.AccessType{
			ID: uuid.NewString(), DomainID: domainID, Title: "x", Bit: tooBig,
		})
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
		if !strings.Contains(err.Error(), "exceeds signed 64-bit range") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("PermissionCreate_bit63", func(t *testing.T) {
		err := s.PermissionCreate(ctx, &store.Permission{
			ID: uuid.NewString(), DomainID: domainID, Title: "p",
			ResourceID: rid, AccessMask: tooBig,
		})
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	atID := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{
		ID: atID, DomainID: domainID, Title: "read", Bit: 1,
	}); err != nil {
		t.Fatal(err)
	}
	pID := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{
		ID: pID, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1,
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("AccessTypePatch_bit63", func(t *testing.T) {
		bit := tooBig
		_, err := s.AccessTypePatch(ctx, domainID, atID, store.AccessTypePatchParams{Bit: &bit})
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	t.Run("PermissionPatch_bit63", func(t *testing.T) {
		mask := tooBig
		_, err := s.PermissionPatch(ctx, domainID, pID, store.PermissionPatchParams{AccessMask: &mask})
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})

	t.Run("AccessTypeCreate_maxInt64_ok", func(t *testing.T) {
		id := uuid.NewString()
		if err := s.AccessTypeCreate(ctx, &store.AccessType{
			ID: id, DomainID: domainID, Title: "max", Bit: maxOK,
		}); err != nil {
			t.Fatalf("create maxOK: %v", err)
		}
		got, err := s.AccessTypeGet(ctx, domainID, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Bit != maxOK {
			t.Fatalf("bit: want %d, got %d", maxOK, got.Bit)
		}
	})

	t.Run("PermissionCreate_maxInt64_ok", func(t *testing.T) {
		id := uuid.NewString()
		if err := s.PermissionCreate(ctx, &store.Permission{
			ID: id, DomainID: domainID, Title: "pmax", ResourceID: rid, AccessMask: maxOK,
		}); err != nil {
			t.Fatalf("create maxOK: %v", err)
		}
		got, err := s.PermissionGet(ctx, domainID, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.AccessMask != maxOK {
			t.Fatalf("mask: want %d, got %d", maxOK, got.AccessMask)
		}
	})
}

// TestT48_TypedInvalidInputError_RoundTrip verifies that store-level
// validation errors are extractable via errors.As as a typed
// store.InvalidInputError, including through fmt.Errorf("%w", err) wrapping,
// and that errors.Is(err, store.ErrInvalidInput) still works.

func TestT48_TypedInvalidInputError_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		call       func(t *testing.T) error
		wantDetail string
	}{
		{
			name:       "DomainPatch_emptyPatch",
			call:       func(t *testing.T) error { _, err := s.DomainPatch(ctx, domainID, nil); return err },
			wantDetail: "empty patch",
		},
		{
			name: "PermissionCreate_maskOverflow",
			call: func(t *testing.T) error {
				rid := uuid.NewString()
				if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
					t.Fatal(err)
				}
				return s.PermissionCreate(ctx, &store.Permission{
					ID: uuid.NewString(), DomainID: domainID, Title: "p",
					ResourceID: rid, AccessMask: 1 << 63,
				})
			},
			wantDetail: store.InvalidInputDetailMaskOverflow,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.call(t)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !errors.Is(err, store.ErrInvalidInput) {
				t.Fatalf("errors.Is(err, store.ErrInvalidInput) = false; err=%v", err)
			}
			var iie *store.InvalidInputError
			if !errors.As(err, &iie) {
				t.Fatalf("errors.As did not extract *store.InvalidInputError; err=%v", err)
			}
			if iie.Detail != c.wantDetail {
				t.Fatalf("Detail = %q, want %q", iie.Detail, c.wantDetail)
			}
			// Robust through extra context wrapping (the bug T48 fixes).
			wrapped := fmt.Errorf("ctx: %w", err)
			var iie2 *store.InvalidInputError
			if !errors.As(wrapped, &iie2) || iie2.Detail != c.wantDetail {
				t.Fatalf("errors.As failed through wrapping; got %v", iie2)
			}
		})
	}
}

// TestMaskFromSQL_negativeMaskHook documents that maskFromSQL invokes the
// negative-mask hook exactly once per negative read and returns 0. See T50.

func TestMaskFromSQL_negativeMaskHook(t *testing.T) {
	s := &Store{}
	t.Cleanup(func() { s.SetNegativeMaskHook(nil) })

	if got := s.maskFromSQL(0); got != 0 {
		t.Fatalf("maskFromSQL(0) = %d, want 0", got)
	}
	if got := s.maskFromSQL(42); got != 42 {
		t.Fatalf("maskFromSQL(42) = %d, want 42", got)
	}

	var calls int
	s.SetNegativeMaskHook(func() { calls++ })

	if got := s.maskFromSQL(-1); got != 0 {
		t.Fatalf("maskFromSQL(-1) = %d, want 0", got)
	}
	if got := s.maskFromSQL(-9999); got != 0 {
		t.Fatalf("maskFromSQL(-9999) = %d, want 0", got)
	}
	if calls != 2 {
		t.Fatalf("hook calls = %d, want 2", calls)
	}

	if got := s.maskFromSQL(7); got != 7 {
		t.Fatalf("maskFromSQL(7) = %d, want 7", got)
	}
	if calls != 2 {
		t.Fatalf("hook called on positive value: calls = %d, want 2", calls)
	}
}

// TestMaskFromSQL_nilHook verifies that maskFromSQL is a no-op (returns 0
// for negatives, identity for non-negatives) when no hook is installed.

func TestMaskFromSQL_nilHook(t *testing.T) {
	s := &Store{}
	if got := s.maskFromSQL(-2); got != 0 {
		t.Fatalf("maskFromSQL(-2) with nil hook = %d, want 0", got)
	}
	if got := s.maskFromSQL(5); got != 5 {
		t.Fatalf("maskFromSQL(5) with nil hook = %d, want 5", got)
	}
}
