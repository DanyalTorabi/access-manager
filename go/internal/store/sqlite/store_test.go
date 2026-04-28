package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open("file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

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

func TestDomainGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "alpha"}); err != nil {
		t.Fatal(err)
	}
	d, err := s.DomainGet(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != id || d.Title != "alpha" {
		t.Fatalf("got %+v", d)
	}
	_, err = s.DomainGet(ctx, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDomainList_emptyAndMultiple(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.DomainList(ctx, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	d1 := uuid.NewString()
	d2 := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: d1, Title: "zebra"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: d2, Title: "apple"}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.DomainList(ctx, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "apple" || list[1].Title != "zebra" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestUserGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "alice"}); err != nil {
		t.Fatal(err)
	}
	u, err := s.UserGet(ctx, domainID, uid)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != uid || u.DomainID != domainID || u.Title != "alice" {
		t.Fatalf("got %+v", u)
	}
	_, err = s.UserGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	_, err = s.UserGet(ctx, uuid.NewString(), uid)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong domain: want ErrNotFound, got %v", err)
	}
}

func TestUserList_emptyAndWithItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	otherDomain := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: otherDomain, Title: "other"}); err != nil {
		t.Fatal(err)
	}
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.UserList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	u1 := uuid.NewString()
	u2 := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: u1, DomainID: domainID, Title: "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: u2, DomainID: domainID, Title: "ann"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: otherDomain, Title: "loner"}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.UserList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "ann" || list[1].Title != "bob" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestGroupGet_foundWithAndWithoutParent_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rootID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: rootID, DomainID: domainID, Title: "root"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "child", ParentGroupID: &rootID}); err != nil {
		t.Fatal(err)
	}
	gRoot, err := s.GroupGet(ctx, domainID, rootID)
	if err != nil {
		t.Fatal(err)
	}
	if gRoot.ParentGroupID != nil {
		t.Fatalf("root should have nil parent, got %+v", gRoot.ParentGroupID)
	}
	gChild, err := s.GroupGet(ctx, domainID, childID)
	if err != nil {
		t.Fatal(err)
	}
	if gChild.ParentGroupID == nil || *gChild.ParentGroupID != rootID {
		t.Fatalf("want parent %s, got %+v", rootID, gChild.ParentGroupID)
	}
	_, err = s.GroupGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGroupList_emptyWithItemsIncludingParent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	allOpts := store.GroupListOpts{ListOpts: store.ListOpts{Offset: 0, Limit: 100}}
	list, total, err := s.GroupList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	parentID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "P"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "C", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.GroupList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	// ORDER BY title: C before P
	if list[0].ID != childID || list[1].ID != parentID {
		t.Fatalf("unexpected order or ids: %+v", list)
	}
	if list[0].ParentGroupID == nil || *list[0].ParentGroupID != parentID {
		t.Fatalf("child list row: want parent %s, got %+v", parentID, list[0].ParentGroupID)
	}
	if list[1].ParentGroupID != nil {
		t.Fatalf("parent row should have nil ParentGroupID, got %+v", list[1].ParentGroupID)
	}
}

func TestGroupSetParent(t *testing.T) {
	ctx := context.Background()

	t.Run("setParentSuccess", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		parentID := uuid.NewString()
		childID := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "par"}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "chi"}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupSetParent(ctx, domainID, childID, &parentID); err != nil {
			t.Fatal(err)
		}
		g, err := s.GroupGet(ctx, domainID, childID)
		if err != nil {
			t.Fatal(err)
		}
		if g.ParentGroupID == nil || *g.ParentGroupID != parentID {
			t.Fatalf("want parent %s, got %+v", parentID, g.ParentGroupID)
		}
	})

	t.Run("clearParent", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		parentID := uuid.NewString()
		childID := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "par"}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "chi", ParentGroupID: &parentID}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupSetParent(ctx, domainID, childID, nil); err != nil {
			t.Fatal(err)
		}
		g, err := s.GroupGet(ctx, domainID, childID)
		if err != nil {
			t.Fatal(err)
		}
		if g.ParentGroupID != nil {
			t.Fatalf("want nil parent, got %+v", g.ParentGroupID)
		}
	})

	t.Run("selfParent", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		gid := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
			t.Fatal(err)
		}
		err := s.GroupSetParent(ctx, domainID, gid, &gid)
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
		if !strings.Contains(err.Error(), "own parent") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		g1 := uuid.NewString()
		g2 := uuid.NewString()
		g3 := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: g1, DomainID: domainID, Title: "g1"}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupCreate(ctx, &store.Group{ID: g2, DomainID: domainID, Title: "g2", ParentGroupID: &g1}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupCreate(ctx, &store.Group{ID: g3, DomainID: domainID, Title: "g3", ParentGroupID: &g2}); err != nil {
			t.Fatal(err)
		}
		// g1 -> g2 -> g3; setting g1's parent to g3 closes the cycle.
		err := s.GroupSetParent(ctx, domainID, g1, &g3)
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nonExistentGroup", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		parentID := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "p"}); err != nil {
			t.Fatal(err)
		}
		err := s.GroupSetParent(ctx, domainID, uuid.NewString(), &parentID)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("nonExistentParent", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		childID := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "c"}); err != nil {
			t.Fatal(err)
		}
		fakeParent := uuid.NewString()
		err := s.GroupSetParent(ctx, domainID, childID, &fakeParent)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestResourceGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "doc"}); err != nil {
		t.Fatal(err)
	}
	r, err := s.ResourceGet(ctx, domainID, rid)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != rid || r.DomainID != domainID || r.Title != "doc" {
		t.Fatalf("got %+v", r)
	}
	_, err = s.ResourceGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestResourceList_emptyAndWithItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	other := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: other, Title: "o"}); err != nil {
		t.Fatal(err)
	}
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.ResourceList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: other, Title: "x"}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.ResourceList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "a" || list[1].Title != "z" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestAccessTypeCreateAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.AccessTypeList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	a1 := uuid.NewString()
	a2 := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a1, DomainID: domainID, Title: "write", Bit: 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a2, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.AccessTypeList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	// ORDER BY bit: read(1) then write(4)
	if list[0].Bit != 1 || list[0].ID != a2 || list[1].Bit != 4 || list[1].ID != a1 {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestPermissionGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "perm", ResourceID: rid, AccessMask: 0x5}); err != nil {
		t.Fatal(err)
	}
	p, err := s.PermissionGet(ctx, domainID, pid)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != pid || p.ResourceID != rid || p.AccessMask != 0x5 {
		t.Fatalf("got %+v", p)
	}
	_, err = s.PermissionGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPermissionList_emptyAndWithItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	other := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: other, Title: "o"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	allOpts := store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 0, Limit: 100}}
	list, total, err := s.PermissionList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	p1 := uuid.NewString()
	p2 := uuid.NewString()
	roid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: roid, DomainID: other, Title: "r2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: p1, DomainID: domainID, Title: "zebra", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: p2, DomainID: domainID, Title: "apple", ResourceID: rid, AccessMask: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: other, Title: "other", ResourceID: roid, AccessMask: 3}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.PermissionList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "apple" || list[1].Title != "zebra" {
		t.Fatalf("order by title: got %+v", list)
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

func TestDomainDelete_emptyDomain(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainDelete(ctx, domainID); err != nil {
		t.Fatal(err)
	}
	_, err := s.DomainGet(ctx, domainID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPatchDomainUserResource(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	title := "d2"
	d, err := s.DomainPatch(ctx, domainID, &title)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "d2" {
		t.Fatalf("domain title: %q", d.Title)
	}
	uid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	ut := "alice"
	u, err := s.UserPatch(ctx, domainID, uid, &ut)
	if err != nil {
		t.Fatal(err)
	}
	if u.Title != "alice" {
		t.Fatalf("user title: %q", u.Title)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	rt := "doc"
	r, err := s.ResourcePatch(ctx, domainID, rid, &rt)
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "doc" {
		t.Fatalf("resource title: %q", r.Title)
	}
}

func TestAccessTypeGetPatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	aid := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: aid, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	got, err := s.AccessTypeGet(ctx, domainID, aid)
	if err != nil || got.Title != "read" || got.Bit != 1 {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	nt := "READ"
	a, err := s.AccessTypePatch(ctx, domainID, aid, store.AccessTypePatchParams{Title: &nt})
	if err != nil || a.Title != "READ" || a.Bit != 1 {
		t.Fatalf("patch title: %+v err=%v", a, err)
	}
	b2 := uint64(2)
	a2, err := s.AccessTypePatch(ctx, domainID, aid, store.AccessTypePatchParams{Bit: &b2})
	if err != nil || a2.Bit != 2 {
		t.Fatalf("patch bit: %+v err=%v", a2, err)
	}
	if err := s.AccessTypeDelete(ctx, domainID, aid); err != nil {
		t.Fatal(err)
	}
	_, err = s.AccessTypeGet(ctx, domainID, aid)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPermissionPatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "b"}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	pt := "perm"
	p, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{Title: &pt})
	if err != nil || p.Title != "perm" {
		t.Fatalf("patch title: %+v err=%v", p, err)
	}
	p, err = s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{ResourceID: &r2})
	if err != nil || p.ResourceID != r2 {
		t.Fatalf("patch resource: %+v err=%v", p, err)
	}
	m := uint64(7)
	p, err = s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{AccessMask: &m})
	if err != nil || p.AccessMask != 7 {
		t.Fatalf("patch mask: %+v err=%v", p, err)
	}
	if err := s.PermissionDelete(ctx, domainID, pid); err != nil {
		t.Fatal(err)
	}
	_, err = s.PermissionGet(ctx, domainID, pid)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGroupPatch_titleAndParent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	pID := uuid.NewString()
	cID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: pID, DomainID: domainID, Title: "par"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: cID, DomainID: domainID, Title: "chi"}); err != nil {
		t.Fatal(err)
	}
	nt := "child"
	g, err := s.GroupPatch(ctx, domainID, cID, store.GroupPatchParams{Title: &nt, UpdateParent: true, ParentGroupID: &pID})
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "child" || g.ParentGroupID == nil || *g.ParentGroupID != pID {
		t.Fatalf("group: %+v", g)
	}
	g, err = s.GroupPatch(ctx, domainID, cID, store.GroupPatchParams{UpdateParent: true, ParentGroupID: nil})
	if err != nil || g.ParentGroupID != nil {
		t.Fatalf("clear parent: %+v err=%v", g, err)
	}
}

func TestDelete_userGroupResource_success(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserDelete(ctx, domainID, uid); err != nil {
		t.Fatalf("UserDelete: %v", err)
	}
	if err := s.GroupDelete(ctx, domainID, gid); err != nil {
		t.Fatalf("GroupDelete: %v", err)
	}
	if err := s.ResourceDelete(ctx, domainID, rid); err != nil {
		t.Fatalf("ResourceDelete: %v", err)
	}
}

func TestDelete_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	missing := uuid.NewString()
	if err := s.DomainDelete(ctx, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DomainDelete: want ErrNotFound, got %v", err)
	}
	if err := s.UserDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("UserDelete: want ErrNotFound, got %v", err)
	}
	if err := s.GroupDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GroupDelete: want ErrNotFound, got %v", err)
	}
	if err := s.ResourceDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ResourceDelete: want ErrNotFound, got %v", err)
	}
	if err := s.AccessTypeDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("AccessTypeDelete: want ErrNotFound, got %v", err)
	}
	if err := s.PermissionDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PermissionDelete: want ErrNotFound, got %v", err)
	}
}

func TestPatch_emptyInvalid_notFound(t *testing.T) {
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
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	aid := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: aid, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DomainPatch(ctx, domainID, nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("DomainPatch nil: want ErrInvalidInput, got %v", err)
	}
	badDomain := uuid.NewString()
	title := "x"
	if _, err := s.DomainPatch(ctx, badDomain, &title); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DomainPatch missing domain: %v", err)
	}
	if _, err := s.UserPatch(ctx, domainID, uuid.NewString(), &title); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("UserPatch not found: %v", err)
	}
	if _, err := s.UserPatch(ctx, domainID, uid, nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("UserPatch nil title: %v", err)
	}
	if _, err := s.ResourcePatch(ctx, domainID, uuid.NewString(), &title); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ResourcePatch not found: %v", err)
	}
	if _, err := s.ResourcePatch(ctx, domainID, rid, nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("ResourcePatch nil: %v", err)
	}
	if _, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("GroupPatch empty: %v", err)
	}
	if _, err := s.GroupPatch(ctx, domainID, uuid.NewString(), store.GroupPatchParams{Title: &title}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GroupPatch missing group: %v", err)
	}
	if _, err := s.AccessTypePatch(ctx, domainID, aid, store.AccessTypePatchParams{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("AccessTypePatch empty: %v", err)
	}
	if _, err := s.AccessTypePatch(ctx, domainID, uuid.NewString(), store.AccessTypePatchParams{Title: &title}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("AccessTypePatch not found: %v", err)
	}
	if _, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("PermissionPatch empty: %v", err)
	}
	if _, err := s.PermissionPatch(ctx, domainID, uuid.NewString(), store.PermissionPatchParams{Title: &title}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PermissionPatch not found: %v", err)
	}
	otherDomain := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: otherDomain, Title: "o"}); err != nil {
		t.Fatal(err)
	}
	otherRes := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: otherRes, DomainID: otherDomain, Title: "or"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{ResourceID: &otherRes}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PermissionPatch foreign resource: want ErrNotFound, got %v", err)
	}
}

func TestGroupPatch_titleOnly(t *testing.T) {
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
	nt := "renamed"
	g, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{Title: &nt})
	if err != nil || g.Title != "renamed" {
		t.Fatalf("got %+v err=%v", g, err)
	}
}

func TestAccessTypePatch_duplicateBitConflict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	a1 := uuid.NewString()
	a2 := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a1, DomainID: domainID, Title: "a", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a2, DomainID: domainID, Title: "b", Bit: 2}); err != nil {
		t.Fatal(err)
	}
	b1 := uint64(2)
	_, err := s.AccessTypePatch(ctx, domainID, a1, store.AccessTypePatchParams{Bit: &b1})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestDomainCreate_duplicateID_conflict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "a"}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "b"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
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

func TestWrapConstraintError_plainErrorUnchanged(t *testing.T) {
	err := wrapConstraintError(errors.New("some other failure"))
	if err == nil || !strings.Contains(err.Error(), "some other failure") {
		t.Fatalf("got %v", err)
	}
	if errors.Is(err, store.ErrFKViolation) || errors.Is(err, store.ErrConflict) {
		t.Fatal("plain error should not be classified as FK/conflict")
	}
}

func TestDomainList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Offset: 0, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("total: want 5, got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("items: want 2, got %d", len(list))
	}
	if list[0].Title != "a" || list[1].Title != "b" {
		t.Fatalf("first page: %+v", list)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Offset: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("page 2: items=%d total=%d", len(list), total)
	}
	if list[0].Title != "d" || list[1].Title != "e" {
		t.Fatalf("page 2 content: %+v", list)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Offset: 10, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 0 {
		t.Fatalf("past end: items=%d total=%d", len(list), total)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Offset: 0, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 1 || list[0].Title != "a" {
		t.Fatalf("limit 1: items=%d total=%d list=%+v", len(list), total, list)
	}
}

func TestUserList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.UserList(ctx, domainID, store.ListOpts{Offset: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}
	if list[0].Title != "b" || list[1].Title != "c" {
		t.Fatalf("content: %+v", list)
	}
}

func TestGroupList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Offset: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}
	if list[0].Title != "b" || list[1].Title != "c" {
		t.Fatalf("content: %+v", list)
	}

	list, total, err = s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Offset: 10, Limit: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 0 {
		t.Fatalf("past end: items=%d total=%d", len(list), total)
	}
}

func TestResourceList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.ResourceList(ctx, domainID, store.ListOpts{Offset: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}
	if list[0].Title != "c" || list[1].Title != "d" {
		t.Fatalf("content: %+v", list)
	}
}

func TestAccessTypeList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		bit := uint64(1 << i)
		if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: title, Bit: bit}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Offset: 0, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 3 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}

	list, total, err = s.AccessTypeList(ctx, domainID, store.ListOpts{Offset: 4, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 1 {
		t.Fatalf("last page: items=%d total=%d", len(list), total)
	}
}

func TestPermissionList_pagination(t *testing.T) {
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
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.PermissionCreate(ctx, &store.Permission{
			ID: uuid.NewString(), DomainID: domainID, Title: title, ResourceID: rid, AccessMask: uint64(1 << i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}

	list, total, err = s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 10, Limit: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 0 {
		t.Fatalf("past end: items=%d total=%d", len(list), total)
	}
}

func TestStore_closedDB_methods(t *testing.T) {
	ctx := context.Background()
	db, err := Open("file:" + filepath.Join(t.TempDir(), "closedall.db") + "?_pragma=foreign_keys(1)")
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
	gid := uuid.NewString()
	rid := uuid.NewString()
	pid := uuid.NewString()
	atID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: atID, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	title := "x"

	t.Run("DomainGet", func(t *testing.T) {
		if _, err := s.DomainGet(ctx, domainID); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("DomainList", func(t *testing.T) {
		if _, _, err := s.DomainList(ctx, allOpts); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("DomainCreate", func(t *testing.T) {
		if err := s.DomainCreate(ctx, &store.Domain{ID: "x", Title: "x"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("DomainDelete", func(t *testing.T) {
		if err := s.DomainDelete(ctx, domainID); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("DomainPatch", func(t *testing.T) {
		if _, err := s.DomainPatch(ctx, domainID, &title); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("UserGet", func(t *testing.T) {
		if _, err := s.UserGet(ctx, domainID, uid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("UserList", func(t *testing.T) {
		if _, _, err := s.UserList(ctx, domainID, allOpts); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("UserCreate", func(t *testing.T) {
		if err := s.UserCreate(ctx, &store.User{ID: "x", DomainID: domainID, Title: "x"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("UserDelete", func(t *testing.T) {
		if err := s.UserDelete(ctx, domainID, uid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("UserPatch", func(t *testing.T) {
		if _, err := s.UserPatch(ctx, domainID, uid, &title); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GroupGet", func(t *testing.T) {
		if _, err := s.GroupGet(ctx, domainID, gid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GroupList", func(t *testing.T) {
		if _, _, err := s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: allOpts}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GroupCreate", func(t *testing.T) {
		if err := s.GroupCreate(ctx, &store.Group{ID: "x", DomainID: domainID, Title: "x"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GroupDelete", func(t *testing.T) {
		if err := s.GroupDelete(ctx, domainID, gid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GroupPatch", func(t *testing.T) {
		if _, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{Title: &title}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GroupSetParent", func(t *testing.T) {
		if err := s.GroupSetParent(ctx, domainID, gid, nil); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("ResourceGet", func(t *testing.T) {
		if _, err := s.ResourceGet(ctx, domainID, rid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("ResourceList", func(t *testing.T) {
		if _, _, err := s.ResourceList(ctx, domainID, allOpts); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("ResourceCreate", func(t *testing.T) {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: "x", DomainID: domainID, Title: "x"}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("ResourceDelete", func(t *testing.T) {
		if err := s.ResourceDelete(ctx, domainID, rid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("ResourcePatch", func(t *testing.T) {
		if _, err := s.ResourcePatch(ctx, domainID, rid, &title); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("AccessTypeGet", func(t *testing.T) {
		if _, err := s.AccessTypeGet(ctx, domainID, atID); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("AccessTypeList", func(t *testing.T) {
		if _, _, err := s.AccessTypeList(ctx, domainID, allOpts); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("AccessTypeCreate", func(t *testing.T) {
		if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: "x", DomainID: domainID, Title: "x", Bit: 2}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("AccessTypeDelete", func(t *testing.T) {
		if err := s.AccessTypeDelete(ctx, domainID, atID); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("AccessTypePatch", func(t *testing.T) {
		if _, err := s.AccessTypePatch(ctx, domainID, atID, store.AccessTypePatchParams{Title: &title}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("PermissionGet", func(t *testing.T) {
		if _, err := s.PermissionGet(ctx, domainID, pid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("PermissionList", func(t *testing.T) {
		if _, _, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: allOpts}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("PermissionCreate", func(t *testing.T) {
		if err := s.PermissionCreate(ctx, &store.Permission{ID: "x", DomainID: domainID, Title: "x", ResourceID: rid, AccessMask: 1}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("PermissionDelete", func(t *testing.T) {
		if err := s.PermissionDelete(ctx, domainID, pid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("PermissionPatch", func(t *testing.T) {
		if _, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{Title: &title}); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("AddUserToGroup", func(t *testing.T) {
		if err := s.AddUserToGroup(ctx, domainID, uid, gid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("RemoveUserFromGroup", func(t *testing.T) {
		if err := s.RemoveUserFromGroup(ctx, domainID, uid, gid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GrantUserPermission", func(t *testing.T) {
		if err := s.GrantUserPermission(ctx, domainID, uid, pid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("RevokeUserPermission", func(t *testing.T) {
		if err := s.RevokeUserPermission(ctx, domainID, uid, pid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("GrantGroupPermission", func(t *testing.T) {
		if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("RevokeGroupPermission", func(t *testing.T) {
		if err := s.RevokeGroupPermission(ctx, domainID, gid, pid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("PermissionMasksForUserResource", func(t *testing.T) {
		if _, err := s.PermissionMasksForUserResource(ctx, domainID, uid, rid); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("UserAuthzResourcesList", func(t *testing.T) {
		if _, _, err := s.UserAuthzResourcesList(ctx, domainID, uid, allOpts); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestGroupPatch_parentOnlyError(t *testing.T) {
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
	badParent := uuid.NewString()
	_, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{
		UpdateParent:  true,
		ParentGroupID: &badParent,
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound for non-existent parent, got %v", err)
	}
}

func TestAccessTypePatch_bitOnly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	atID := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: atID, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	newBit := uint64(2)
	got, err := s.AccessTypePatch(ctx, domainID, atID, store.AccessTypePatchParams{Bit: &newBit})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bit != 2 {
		t.Fatalf("bit: want 2, got %d", got.Bit)
	}
	if got.Title != "read" {
		t.Fatalf("title should be unchanged, got %q", got.Title)
	}
}

func TestPermissionPatch_maskOnly(t *testing.T) {
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
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	newMask := uint64(0xFF)
	got, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{AccessMask: &newMask})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessMask != 0xFF {
		t.Fatalf("mask: want 0xff, got %#x", got.AccessMask)
	}
	if got.Title != "p" {
		t.Fatalf("title should be unchanged, got %q", got.Title)
	}
}

func TestPermissionPatch_resourceIDOnly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "r1"}); err != nil {
		t.Fatal(err)
	}
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "r2"}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	got, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{ResourceID: &r2})
	if err != nil {
		t.Fatal(err)
	}
	if got.ResourceID != r2 {
		t.Fatalf("resource_id: want %s, got %s", r2, got.ResourceID)
	}
}

func TestSanitizeListOpts(t *testing.T) {
	tests := []struct {
		name string
		in   store.ListOpts
		want store.ListOpts
	}{
		{"zero limit defaults", store.ListOpts{Offset: 0, Limit: 0}, store.ListOpts{Offset: 0, Limit: store.DefaultLimit, Order: store.OrderAsc}},
		{"negative limit defaults", store.ListOpts{Offset: 0, Limit: -5}, store.ListOpts{Offset: 0, Limit: store.DefaultLimit, Order: store.OrderAsc}},
		{"over max capped", store.ListOpts{Offset: 0, Limit: 500}, store.ListOpts{Offset: 0, Limit: store.MaxLimit, Order: store.OrderAsc}},
		{"negative offset zeroed", store.ListOpts{Offset: -3, Limit: 10}, store.ListOpts{Offset: 0, Limit: 10, Order: store.OrderAsc}},
		{"valid unchanged", store.ListOpts{Offset: 5, Limit: 25}, store.ListOpts{Offset: 5, Limit: 25, Order: store.OrderAsc}},
		{"order preserved when set", store.ListOpts{Offset: 0, Limit: 10, Order: store.OrderDesc}, store.ListOpts{Offset: 0, Limit: 10, Order: store.OrderDesc}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.SanitizeListOpts(tt.in)
			if got != tt.want {
				t.Fatalf("SanitizeListOpts(%+v) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDomainList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Alpha", "Beta", "Alphabet"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "alph"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "Alpha" || list[1].Title != "Alphabet" {
		t.Fatalf("unexpected titles: %+v", list)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("want 0, got %d items total=%d", len(list), total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: ""})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("empty search should return all, got total=%d", total)
	}
}

func TestUserList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Alice", "Bob", "Alicia"} {
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.UserList(ctx, domainID, store.ListOpts{Limit: 100, Search: "ali"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}

func TestGroupList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Admins", "Editors", "Admin-sub"} {
		if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts: store.ListOpts{Limit: 100, Search: "admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}

func TestGroupList_filterByParentGroupID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	parentID := uuid.NewString()
	child1 := uuid.NewString()
	child2 := uuid.NewString()
	root2 := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "parent"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: child1, DomainID: domainID, Title: "child1", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: child2, DomainID: domainID, Title: "child2", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: root2, DomainID: domainID, Title: "root2"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts:      store.ListOpts{Limit: 100},
		ParentGroupID: &parentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2 children, got %d items total=%d", len(list), total)
	}
	if list[0].ID != child1 || list[1].ID != child2 {
		t.Fatalf("unexpected children: %+v", list)
	}

	_, total, err = s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts: store.ListOpts{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("no filter should return all 4, got total=%d", total)
	}
}

func TestGroupList_searchAndParentCombined(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	parentID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "root"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: "dev-team", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: "ops-team", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts:      store.ListOpts{Limit: 100, Search: "dev"},
		ParentGroupID: &parentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "dev-team" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}
}

func TestResourceList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Document", "Image", "Documentation"} {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.ResourceList(ctx, domainID, store.ListOpts{Limit: 100, Search: "doc"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}

func TestAccessTypeList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"read", "write", "readonly"} {
		if err := s.AccessTypeCreate(ctx, &store.AccessType{
			ID: uuid.NewString(), DomainID: domainID, Title: title, Bit: uint64(1 << i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Limit: 100, Search: "read"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}

func TestPermissionList_search(t *testing.T) {
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
	for _, title := range []string{"can-read", "can-write", "can-read-all"} {
		if err := s.PermissionCreate(ctx, &store.Permission{
			ID: uuid.NewString(), DomainID: domainID, Title: title, ResourceID: rid, AccessMask: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts: store.ListOpts{Limit: 100, Search: "can-read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}

func TestPermissionList_filterByResourceID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "res1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "res2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p1", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p2", ResourceID: r1, AccessMask: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p3", ResourceID: r2, AccessMask: 4}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts:   store.ListOpts{Limit: 100},
		ResourceID: &r1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2 for r1, got %d items total=%d", len(list), total)
	}

	list, total, err = s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts:   store.ListOpts{Limit: 100},
		ResourceID: &r2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1 for r2, got %d items total=%d", len(list), total)
	}
}

func TestPermissionList_searchAndResourceCombined(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "res1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "res2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "read-doc", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "write-doc", ResourceID: r1, AccessMask: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "read-img", ResourceID: r2, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts:   store.ListOpts{Limit: 100, Search: "read"},
		ResourceID: &r1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "read-doc" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}
}

func TestDomainList_searchWithPagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		title := "test-" + string(rune('a'+i))
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: "other"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 2, Offset: 0, Search: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("total should be 5 (all matching), got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("page size should be 2, got %d", len(list))
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 2, Offset: 4, Search: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 1 {
		t.Fatalf("last page: want total=5 items=1, got total=%d items=%d", total, len(list))
	}
}

func TestDomainList_searchEscapesWildcards(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"100% done", "normal", "test_case"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "%"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("search for literal %%: want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "100% done" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "_"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("search for literal _: want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "test_case" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: `\`})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("search for literal backslash: want 0, got %d items total=%d", len(list), total)
	}
}

func TestDomainList_searchType(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Alpha", "Alphabet", "Beta"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "Alpha", SearchType: store.SearchStartsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("starts_with Alpha: want 2, got %d items total=%d", len(list), total)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "bet", SearchType: store.SearchEndsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("ends_with bet: want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "Alphabet" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "lph", SearchType: store.SearchContains})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("contains lph: want 2, got total=%d", total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "lph"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("empty SearchType (default contains) lph: want 2, got total=%d", total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "Alp", SearchType: store.SearchEndsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("ends_with Alp: want 0, got total=%d", total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "eta", SearchType: store.SearchStartsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("starts_with eta: want 0, got total=%d", total)
	}
}

func TestDomainList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Alpha", "Beta", "Charlie"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3, got %d", len(list))
	}
	if list[0].Title != "Charlie" || list[2].Title != "Alpha" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestDomainList_sortAscExplicit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Charlie", "Alpha", "Beta"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderAsc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Alpha" || list[2].Title != "Charlie" {
		t.Fatalf("asc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestUserList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Alice", "Bob", "Charlie"} {
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.UserList(ctx, domainID, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Charlie" || list[2].Title != "Alice" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestGroupList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Admins", "Editors", "Viewers"} {
		if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc}})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Viewers" || list[2].Title != "Admins" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestResourceList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Doc", "File", "Repo"} {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.ResourceList(ctx, domainID, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Repo" || list[2].Title != "Doc" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestAccessTypeList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"Read", "Write", "Execute"} {
		if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: title, Bit: uint64(1 << i)}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Write" || list[2].Title != "Execute" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestPermissionList_sortDesc(t *testing.T) {
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
	for _, title := range []string{"perm-a", "perm-b", "perm-c"} {
		if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: title, ResourceID: rid, AccessMask: 1}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc}})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "perm-c" || list[2].Title != "perm-a" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

func TestPermissionList_sortByResourceID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rids := []string{uuid.NewString(), uuid.NewString()}
	for i, rid := range rids {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: fmt.Sprintf("r%d", i)}); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p1", ResourceID: rids[1], AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p2", ResourceID: rids[0], AccessMask: 2}); err != nil {
		t.Fatal(err)
	}

	list, _, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "resource_id", Order: store.OrderAsc}})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].ResourceID > list[1].ResourceID {
		t.Fatalf("asc resource_id: %s should come before %s", list[0].ResourceID, list[1].ResourceID)
	}

	list, _, err = s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "resource_id", Order: store.OrderDesc}})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ResourceID < list[1].ResourceID {
		t.Fatalf("desc resource_id: %s should come after %s", list[0].ResourceID, list[1].ResourceID)
	}
}

func TestSortColumns(t *testing.T) {
	t.Run("no overrides", func(t *testing.T) {
		cols := sortColumns([]string{"title", "resource_id"}, nil)
		if len(cols) != 2 {
			t.Fatalf("want 2 entries, got %d", len(cols))
		}
		if cols["title"] != "title" || cols["resource_id"] != "resource_id" {
			t.Fatalf("unexpected mapping: %v", cols)
		}
	})

	t.Run("valid override", func(t *testing.T) {
		cols := sortColumns([]string{"title"}, map[string]string{"title": "name"})
		if cols["title"] != "name" {
			t.Fatalf("want title→name, got title→%s", cols["title"])
		}
	})

	t.Run("invalid override key ignored", func(t *testing.T) {
		cols := sortColumns([]string{"title"}, map[string]string{"unknown": "col"})
		if _, ok := cols["unknown"]; ok {
			t.Fatal("override key not in fields should be ignored")
		}
		if len(cols) != 1 {
			t.Fatalf("want 1 entry, got %d", len(cols))
		}
	})
}

func TestOrderByClause(t *testing.T) {
	allowed := map[string]string{"title": "title", "resource_id": "resource_id"}

	t.Run("known field", func(t *testing.T) {
		got := orderByClause("title", store.OrderAsc, allowed, "title")
		want := " ORDER BY title ASC, id ASC"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("empty defaults to fallback", func(t *testing.T) {
		got := orderByClause("", store.OrderDesc, allowed, "title")
		want := " ORDER BY title DESC, id DESC"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("unknown non-empty falls back with warning", func(t *testing.T) {
		got := orderByClause("bogus", store.OrderAsc, allowed, "title")
		want := " ORDER BY title ASC, id ASC"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("sort by id skips tiebreaker", func(t *testing.T) {
		a := map[string]string{"id": "id"}
		got := orderByClause("id", store.OrderAsc, a, "id")
		want := " ORDER BY id ASC"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestList_queryContextError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open("file:" + filepath.Join(dir, "qctx.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	s := New(db)

	domID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domID, Title: "r"}); err != nil {
		t.Fatal(err)
	}

	opts := store.ListOpts{Limit: 10, Sort: "title", Order: store.OrderAsc}

	dropAndReplace := func(table, viewSQL string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			t.Fatalf("disable FK: %v", err)
		}
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+table); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
		if _, err := db.ExecContext(ctx, viewSQL); err != nil {
			t.Fatalf("create view %s: %v", table, err)
		}
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatalf("enable FK: %v", err)
		}
	}

	t.Run("UserList", func(t *testing.T) {
		dropAndReplace("users", "CREATE VIEW users AS SELECT 'x' AS domain_id")
		_, _, err := s.UserList(ctx, domID, opts)
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("GroupList", func(t *testing.T) {
		dropAndReplace("groups", "CREATE VIEW groups AS SELECT 'x' AS domain_id")
		_, _, err := s.GroupList(ctx, domID, store.GroupListOpts{ListOpts: opts})
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("ResourceList", func(t *testing.T) {
		dropAndReplace("resources", "CREATE VIEW resources AS SELECT 'x' AS domain_id")
		_, _, err := s.ResourceList(ctx, domID, opts)
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("AccessTypeList", func(t *testing.T) {
		dropAndReplace("access_types", "CREATE VIEW access_types AS SELECT 'x' AS domain_id")
		_, _, err := s.AccessTypeList(ctx, domID, opts)
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("PermissionList", func(t *testing.T) {
		dropAndReplace("permissions", "CREATE VIEW permissions AS SELECT 'x' AS domain_id")
		_, _, err := s.PermissionList(ctx, domID, store.PermissionListOpts{ListOpts: opts})
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("DomainList", func(t *testing.T) {
		dropAndReplace("domains", "CREATE VIEW domains AS SELECT 1 AS x")
		_, _, err := s.DomainList(ctx, opts)
		if err == nil {
			t.Fatal("want error")
		}
	})
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

// Create three users:
// uA: direct grant 0x1 + group grant 0x4 -> 0x5
// uB: group grant only 0x2 -> 0x2
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
