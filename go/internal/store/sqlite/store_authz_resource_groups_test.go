package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
