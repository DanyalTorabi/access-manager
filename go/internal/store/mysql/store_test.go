//go:build integration

package mysql

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

// newTestStore creates a new MySQL-backed Store for integration tests.
// It creates a unique database per test, applies migrations, and registers a
// cleanup that drops the database at the end of the test.
//
// The test is skipped if DATABASE_DSN_MYSQL is not set.
// The DSN must point to a MySQL instance where the connecting user has
// CREATE DATABASE / DROP DATABASE privileges.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	baseDSN := os.Getenv("DATABASE_DSN_MYSQL")
	if baseDSN == "" {
		t.Skip("DATABASE_DSN_MYSQL not set; skipping mysql integration tests")
	}

	// Derive a unique database name from the test name.
	dbName := "test_" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, strings.ToLower(t.Name()))
	if len(dbName) > 64 {
		dbName = dbName[:64]
	}

	// Open a connection to create and drop the test database.
	adminDB, err := Open(baseDSN)
	if err != nil {
		t.Fatalf("Open admin DSN: %v", err)
	}
	if _, err := adminDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName)); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create test database: %v", err)
	}
	_ = adminDB.Close()

	t.Cleanup(func() {
		dropDB, err := Open(baseDSN)
		if err == nil {
			_, _ = dropDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
			_ = dropDB.Close()
		}
	})

	// Build DSN pointing at the test database.
	testDSN := appendDBName(baseDSN, dbName)
	db, err := Open(testDSN)
	if err != nil {
		t.Fatalf("Open test DB DSN: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := MigrateUp(db, testutil.MySQLMigrationsDir(t)); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return New(db)
}

// appendDBName replaces or appends the database name in a MySQL DSN.
// MySQL DSNs have the form: [user[:password]@][protocol[(address)]]/dbname[?params]
// We replace dbname to point to the test database.
func appendDBName(dsn, dbName string) string {
	// Find the last '/' before any '?' to locate the dbname position.
	qIdx := strings.Index(dsn, "?")
	searchIn := dsn
	if qIdx >= 0 {
		searchIn = dsn[:qIdx]
	}
	slashIdx := strings.LastIndex(searchIn, "/")
	if slashIdx < 0 {
		// Malformed DSN; return as-is.
		return dsn
	}
	params := ""
	if qIdx >= 0 {
		params = dsn[qIdx:]
	}
	return dsn[:slashIdx+1] + dbName + params
}

func TestMySQL_EffectiveMask_userAndGroup(t *testing.T) {
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

func TestMySQL_EffectiveMask_directUser(t *testing.T) {
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

func TestMySQL_EffectiveMask_noGrants(t *testing.T) {
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

func TestMySQL_DomainConflict(t *testing.T) {
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

func TestMySQL_UserFKViolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: "nonexistent-domain", Title: "u"})
	if !errors.Is(err, store.ErrFKViolation) {
		t.Fatalf("want ErrFKViolation for missing domain, got %v", err)
	}
}

func TestMySQL_DomainNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, err := s.DomainGet(ctx, "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMySQL_DomainCRUD(t *testing.T) {
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

func TestMySQL_AccessType_BigintUnsigned(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	// bit is BIGINT UNSIGNED — values up to 2^63-1 are valid (store.ErrInvalidInput
	// guards the overflow path at the access_mask level, not bit level in MySQL).
	atID := uuid.NewString()
	bit := uint64(1 << 62)
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: atID, DomainID: domainID, Title: "at", Bit: bit}); err != nil {
		t.Fatalf("AccessTypeCreate: %v", err)
	}
	at, err := s.AccessTypeGet(ctx, domainID, atID)
	if err != nil {
		t.Fatal(err)
	}
	if at.Bit != bit {
		t.Fatalf("want Bit=%d, got %d", bit, at.Bit)
	}
}

func TestMySQL_GroupSetParent_cycle(t *testing.T) {
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

func TestMySQL_MaskOverflow(t *testing.T) {
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

func TestMySQL_UserAuthzResourcesList(t *testing.T) {
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
