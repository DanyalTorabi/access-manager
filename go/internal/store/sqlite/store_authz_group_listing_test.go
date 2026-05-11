package sqlite

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
