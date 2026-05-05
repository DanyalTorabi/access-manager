//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// newTestStore creates a new Postgres-backed Store for integration tests.
// It creates a unique schema per test, applies migrations, and registers a
// cleanup that drops the schema at the end of the test.
//
// The test is skipped if DATABASE_DSN_POSTGRES is not set.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN_POSTGRES")
	if dsn == "" {
		t.Skip("DATABASE_DSN_POSTGRES not set; skipping postgres integration tests")
	}

	schemaName := "test_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	// Limit schema name length to 63 chars (Postgres identifier limit).
	if len(schemaName) > 63 {
		schemaName = schemaName[:63]
	}

	// Create the schema using a temporary connection before opening the pool.
	// This avoids any pool-safety concerns with DDL on a shared connection.
	bootstrapDB, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open (bootstrap): %v", err)
	}
	if _, err := bootstrapDB.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, pq.QuoteIdentifier(schemaName))); err != nil {
		_ = bootstrapDB.Close()
		t.Fatalf("create schema: %v", err)
	}
	_ = bootstrapDB.Close()

	// Embed search_path in the DSN so every connection in the pool uses the
	// test schema automatically — SET search_path is session-scoped and not
	// safe to set after Open() when the pool may have multiple connections.
	schemaDSN := dsn
	if strings.Contains(dsn, "?") {
		schemaDSN = dsn + "&search_path=" + schemaName
	} else {
		schemaDSN = dsn + "?search_path=" + schemaName
	}

	db, err := Open(schemaDSN)
	if err != nil {
		// Schema was created; register cleanup via a fresh connection.
		cleanupDB, cErr := Open(dsn)
		if cErr == nil {
			_, _ = cleanupDB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, pq.QuoteIdentifier(schemaName)))
			_ = cleanupDB.Close()
		}
		t.Fatalf("Open (schema DSN): %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, pq.QuoteIdentifier(schemaName)))
		_ = db.Close()
	})

	if err := MigrateUp(db, testutil.PostgresMigrationsDir(t)); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return New(db)
}

func TestPostgres_EffectiveMask_userAndGroup(t *testing.T) {
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

func TestPostgres_EffectiveMask_directUser(t *testing.T) {
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
		t.Fatalf("mask = %#x, want 0x4", m)
	}
}

func TestPostgres_EffectiveMask_noGrants(t *testing.T) {
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

func TestPostgres_DomainConflict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "d1"}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "d2"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict on duplicate domain ID, got %v", err)
	}
}

func TestPostgres_UserFKViolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: "nonexistent-domain", Title: "u"})
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation for missing domain, got %v", err)
	}
}

func TestPostgres_DomainNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.DomainGet(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPostgres_DomainCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := uuid.NewString()

	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "original"}); err != nil {
		t.Fatal(err)
	}
	d, err := s.DomainGet(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "original" {
		t.Fatalf("want title 'original', got %q", d.Title)
	}

	newTitle := "patched"
	d2, err := s.DomainPatch(ctx, id, &newTitle)
	if err != nil {
		t.Fatal(err)
	}
	if d2.Title != "patched" {
		t.Fatalf("want title 'patched', got %q", d2.Title)
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("want total=1, got %d", total)
	}
	if len(list) != 1 || list[0].ID != id {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := s.DomainDelete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DomainGet(ctx, id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_GroupSetParent_cycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	gA := uuid.NewString()
	gB := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gA, DomainID: domainID, Title: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gB, DomainID: domainID, Title: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupSetParent(ctx, domainID, gB, &gA); err != nil {
		t.Fatal(err)
	}
	err := s.GroupSetParent(ctx, domainID, gA, &gB)
	if !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput for cycle, got %v", err)
	}
}

func TestPostgres_MaskOverflow(t *testing.T) {
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
	overflowMask := uint64(1 << 63)
	err := s.PermissionCreate(ctx, &store.Permission{
		ID: uuid.NewString(), DomainID: domainID, Title: "p",
		ResourceID: rid, AccessMask: overflowMask,
	})
	if !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput for mask overflow, got %v", err)
	}
}

func TestPostgres_UserAuthzResourcesList(t *testing.T) {
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
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.UserAuthzResourcesList(ctx, domainID, uid, store.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("want total=1, got %d", total)
	}
	if len(list) != 1 || list[0].ResourceID != rid {
		t.Fatalf("unexpected list: %v", list)
	}
	if list[0].EffectiveMask != 0x1 {
		t.Fatalf("want mask 0x1, got %#x", list[0].EffectiveMask)
	}

	// Unknown user returns ErrNotFound.
	_, _, err = s.UserAuthzResourcesList(ctx, domainID, uuid.NewString(), store.ListOpts{})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown user: want ErrNotFound, got %v", err)
	}
}

func TestPostgres_EffectiveMask_userPlusGroupOR(t *testing.T) {
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

func TestPostgres_UserCRUD(t *testing.T) {
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
	if _, err := s.UserGet(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := s.UserGet(ctx, uuid.NewString(), uid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong domain: want ErrNotFound, got %v", err)
	}

	// Create a second user; list should return both sorted by title.
	uid2 := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid2, DomainID: domainID, Title: "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: otherDomain, Title: "other"}); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.UserList(ctx, domainID, store.ListOpts{Offset: 0, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", total, len(list))
	}
	if list[0].Title != "alice" || list[1].Title != "bob" {
		t.Fatalf("want alice, bob; got %+v", list)
	}

	// Patch title.
	newTitle := "alicia"
	patched, err := s.UserPatch(ctx, domainID, uid, &newTitle)
	if err != nil {
		t.Fatal(err)
	}
	if patched.Title != "alicia" {
		t.Fatalf("want 'alicia', got %q", patched.Title)
	}

	// Delete and verify not found.
	if err := s.UserDelete(ctx, domainID, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserGet(ctx, domainID, uid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
	if err := s.UserDelete(ctx, domainID, uid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: want ErrNotFound, got %v", err)
	}
}

func TestPostgres_GroupCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}

	parentID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "parent"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "child", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}

	// Get root group (no parent).
	gp, err := s.GroupGet(ctx, domainID, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if gp.ParentGroupID != nil {
		t.Fatalf("root should have nil parent, got %+v", gp.ParentGroupID)
	}

	// Get child (has parent).
	gc, err := s.GroupGet(ctx, domainID, childID)
	if err != nil {
		t.Fatal(err)
	}
	if gc.ParentGroupID == nil || *gc.ParentGroupID != parentID {
		t.Fatalf("want parent %s, got %+v", parentID, gc.ParentGroupID)
	}

	if _, err := s.GroupGet(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// List: child (C) before parent (P) by title.
	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Offset: 0, Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", total, len(list))
	}
	if list[0].ID != childID || list[1].ID != parentID {
		t.Fatalf("order by title: got %+v", list)
	}

	// Patch.
	newTitle := "updated"
	patched, err := s.GroupPatch(ctx, domainID, childID, store.GroupPatchParams{Title: &newTitle})
	if err != nil {
		t.Fatal(err)
	}
	if patched.Title != "updated" {
		t.Fatalf("want 'updated', got %q", patched.Title)
	}

	// Delete child then parent (child must go first due to FK).
	if err := s.GroupDelete(ctx, domainID, childID); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupDelete(ctx, domainID, parentID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GroupGet(ctx, domainID, parentID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_GroupSetParent_clearAndSelf(t *testing.T) {
	ctx := context.Background()

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
			t.Fatalf("want nil parent after clear, got %+v", g.ParentGroupID)
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
			t.Fatalf("want ErrInvalidInput for self-parent, got %v", err)
		}
	})
}

func TestPostgres_ResourceCRUD(t *testing.T) {
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
	if _, err := s.ResourceGet(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	rid2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid2, DomainID: domainID, Title: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: other, Title: "isolated"}); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.ResourceList(ctx, domainID, store.ListOpts{Offset: 0, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", total, len(list))
	}
	if list[0].Title != "alpha" || list[1].Title != "doc" {
		t.Fatalf("order by title: got %+v", list)
	}

	newTitle := "updated"
	if _, err := s.ResourcePatch(ctx, domainID, rid, &newTitle); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceDelete(ctx, domainID, rid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResourceGet(ctx, domainID, rid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_AccessTypeCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}

	a1 := uuid.NewString()
	a2 := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a1, DomainID: domainID, Title: "write", Bit: 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a2, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}

	at, err := s.AccessTypeGet(ctx, domainID, a1)
	if err != nil {
		t.Fatal(err)
	}
	if at.Bit != 4 || at.Title != "write" {
		t.Fatalf("got %+v", at)
	}
	if _, err := s.AccessTypeGet(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	list, total, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Offset: 0, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", total, len(list))
	}
	if list[0].Title != "read" || list[1].Title != "write" {
		t.Fatalf("order by title: got %+v", list)
	}

	newTitle := "execute"
	newBit := uint64(8)
	patched, err := s.AccessTypePatch(ctx, domainID, a1, store.AccessTypePatchParams{Title: &newTitle, Bit: &newBit})
	if err != nil {
		t.Fatal(err)
	}
	if patched.Title != "execute" || patched.Bit != 8 {
		t.Fatalf("patch: got %+v", patched)
	}

	if err := s.AccessTypeDelete(ctx, domainID, a1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccessTypeGet(ctx, domainID, a1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_PermissionCRUD(t *testing.T) {
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
	if _, err := s.PermissionGet(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	pid2 := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid2, DomainID: domainID, Title: "apple", ResourceID: rid, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 0, Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", total, len(list))
	}
	if list[0].Title != "apple" || list[1].Title != "perm" {
		t.Fatalf("order by title: got %+v", list)
	}

	newTitle := "patched"
	newMask := uint64(0x7)
	if _, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{Title: &newTitle, AccessMask: &newMask}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionDelete(ctx, domainID, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PermissionGet(ctx, domainID, pid); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestPostgres_AddUserToGroup_FK(t *testing.T) {
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

func TestPostgres_GrantUserPermission_FK(t *testing.T) {
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

func TestPostgres_GrantGroupPermission_FK(t *testing.T) {
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

func TestPostgres_AddUserToGroup_duplicate(t *testing.T) {
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
	if err := s.AddUserToGroup(ctx, domainID, uid, gid); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestPostgres_GrantUserPermission_duplicate(t *testing.T) {
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
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestPostgres_GrantGroupPermission_duplicate(t *testing.T) {
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
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestPostgres_RemoveUserFromGroup(t *testing.T) {
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
}

func TestPostgres_RevokeUserPermission(t *testing.T) {
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

func TestPostgres_RevokeGroupPermission(t *testing.T) {
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

func TestPostgres_RestrictDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("domainWithUser", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: "u"}); err != nil {
			t.Fatal(err)
		}
		if err := s.DomainDelete(ctx, domainID); !errors.Is(err, store.ErrFKViolation) {
			t.Fatalf("want ErrFKViolation, got %v", err)
		}
	})

	t.Run("resourceWithPermission", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		rid := uuid.NewString()
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
			t.Fatal(err)
		}
		if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
			t.Fatal(err)
		}
		if err := s.ResourceDelete(ctx, domainID, rid); !errors.Is(err, store.ErrFKViolation) {
			t.Fatalf("want ErrFKViolation, got %v", err)
		}
	})

	t.Run("userInGroup", func(t *testing.T) {
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
		if err := s.UserDelete(ctx, domainID, uid); !errors.Is(err, store.ErrFKViolation) {
			t.Fatalf("want ErrFKViolation, got %v", err)
		}
	})

	t.Run("groupWithChild", func(t *testing.T) {
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
		if err := s.GroupDelete(ctx, domainID, parentID); !errors.Is(err, store.ErrFKViolation) {
			t.Fatalf("want ErrFKViolation, got %v", err)
		}
	})
}

func TestPostgres_GroupAuthzResourcesList(t *testing.T) {
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
	for _, rid := range []string{ridA, ridB} {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r-" + rid}); err != nil {
			t.Fatal(err)
		}
	}

	pA1 := uuid.NewString()
	pA2 := uuid.NewString()
	pB := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pA1, DomainID: domainID, Title: "pA1", ResourceID: ridA, AccessMask: 0x1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pA2, DomainID: domainID, Title: "pA2", ResourceID: ridA, AccessMask: 0x4}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pB, DomainID: domainID, Title: "pB", ResourceID: ridB, AccessMask: 0x2}); err != nil {
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

	// Unknown group returns ErrNotFound.
	_, _, err = s.GroupAuthzResourcesList(ctx, domainID, uuid.NewString(), store.ListOpts{})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown group: want ErrNotFound, got %v", err)
	}
}

func TestPostgres_ResourceAuthzUsersList(t *testing.T) {
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
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x3}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantUserPermission(ctx, domainID, uid, pid); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzUsersList(ctx, domainID, rid, store.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1, got total=%d len=%d", total, len(list))
	}
	if list[0].UserID != uid || list[0].EffectiveMask != 0x3 {
		t.Fatalf("got %+v", list[0])
	}

	// Unknown resource returns ErrNotFound.
	_, _, err = s.ResourceAuthzUsersList(ctx, domainID, uuid.NewString(), store.ListOpts{})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown resource: want ErrNotFound, got %v", err)
	}
}

func TestPostgres_ResourceAuthzGroupsList(t *testing.T) {
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
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 0x2}); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantGroupPermission(ctx, domainID, gid, pid); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.ResourceAuthzGroupsList(ctx, domainID, rid, store.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1, got total=%d len=%d", total, len(list))
	}
	if list[0].GroupID != gid || list[0].Mask != 0x2 {
		t.Fatalf("got %+v", list[0])
	}

	// Unknown resource returns ErrNotFound.
	_, _, err = s.ResourceAuthzGroupsList(ctx, domainID, uuid.NewString(), store.ListOpts{})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown resource: want ErrNotFound, got %v", err)
	}
}

func TestPostgres_PermissionMasksForUserResource(t *testing.T) {
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
	pUser := uuid.NewString()
	pGroup := uuid.NewString()
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

	masks, err := s.PermissionMasksForUserResource(ctx, domainID, uid, rid)
	if err != nil {
		t.Fatal(err)
	}
	if len(masks) != 2 {
		t.Fatalf("want 2 masks (user + group), got %d: %v", len(masks), masks)
	}
	combined := uint64(0)
	for _, m := range masks {
		combined |= m
	}
	if combined != 0x3 {
		t.Fatalf("want combined mask 0x3, got %#x", combined)
	}
}

func TestPostgres_DomainList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: fmt.Sprintf("d%02d", i)}); err != nil {
			t.Fatal(err)
		}
	}

	// First page.
	page1, total, err := s.DomainList(ctx, store.ListOpts{Offset: 0, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("want total=5, got %d", total)
	}
	if len(page1) != 3 {
		t.Fatalf("want 3 items, got %d", len(page1))
	}

	// Second page.
	page2, _, err := s.DomainList(ctx, store.ListOpts{Offset: 3, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 2 {
		t.Fatalf("want 2 items on page 2, got %d", len(page2))
	}
}

func TestPostgres_DomainList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: "alphabet"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: "beta"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("search 'alpha': want 2, got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("search 'alpha': want 2 items, got %d", len(list))
	}
	if list[0].Title != "alpha" || list[1].Title != "alphabet" {
		t.Fatalf("want [alpha alphabet], got %+v", list)
	}
}

func TestPostgres_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}

	if err := s.DomainDelete(ctx, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("domain delete not found: want ErrNotFound, got %v", err)
	}
	if err := s.UserDelete(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("user delete not found: want ErrNotFound, got %v", err)
	}
	if err := s.GroupDelete(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("group delete not found: want ErrNotFound, got %v", err)
	}
	if err := s.ResourceDelete(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("resource delete not found: want ErrNotFound, got %v", err)
	}
	if err := s.AccessTypeDelete(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("access type delete not found: want ErrNotFound, got %v", err)
	}
	if err := s.PermissionDelete(ctx, domainID, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("permission delete not found: want ErrNotFound, got %v", err)
	}
}
