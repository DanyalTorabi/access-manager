package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

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
