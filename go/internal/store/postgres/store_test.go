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

	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %q`, schemaName)); err != nil {
		_ = db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO %q`, schemaName)); err != nil {
		_ = db.Close()
		t.Fatalf("set search_path: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schemaName))
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
	if total < 1 {
		t.Fatalf("want total >= 1, got %d", total)
	}
	_ = list

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
}
