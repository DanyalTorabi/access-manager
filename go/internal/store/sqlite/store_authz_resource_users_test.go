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
