package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
