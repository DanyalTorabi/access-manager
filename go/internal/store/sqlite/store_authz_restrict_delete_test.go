package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
