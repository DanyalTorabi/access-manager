package sqlite

import (
	"context"
	"testing"
	"errors"
	"path/filepath"
	"strings"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func TestWrapConstraintError_plainErrorUnchanged(t *testing.T) {
	err := wrapConstraintError(errors.New("some other failure"))
	if err == nil || !strings.Contains(err.Error(), "some other failure") {
		t.Fatalf("got %v", err)
	}
	if errors.Is(err, store.ErrFKViolation) || errors.Is(err, store.ErrConflict) {
		t.Fatal("plain error should not be classified as FK/conflict")
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

