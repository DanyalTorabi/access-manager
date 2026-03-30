package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dtorabi/access-manager/internal/access"
	"github.com/dtorabi/access-manager/internal/store"
	"github.com/dtorabi/access-manager/internal/testutil"
	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open("file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := MigrateUp(db, testutil.SQLiteMigrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

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

func TestDomainGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "alpha"}); err != nil {
		t.Fatal(err)
	}
	d, err := s.DomainGet(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != id || d.Title != "alpha" {
		t.Fatalf("got %+v", d)
	}
	_, err = s.DomainGet(ctx, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDomainList_emptyAndMultiple(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	list, err := s.DomainList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
	d1 := uuid.NewString()
	d2 := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: d1, Title: "zebra"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: d2, Title: "apple"}); err != nil {
		t.Fatal(err)
	}
	list, err = s.DomainList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].Title != "apple" || list[1].Title != "zebra" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestUserGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
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
	_, err = s.UserGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	_, err = s.UserGet(ctx, uuid.NewString(), uid)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong domain: want ErrNotFound, got %v", err)
	}
}

func TestUserList_emptyAndWithItems(t *testing.T) {
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
	list, err := s.UserList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
	u1 := uuid.NewString()
	u2 := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: u1, DomainID: domainID, Title: "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: u2, DomainID: domainID, Title: "ann"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: otherDomain, Title: "loner"}); err != nil {
		t.Fatal(err)
	}
	list, err = s.UserList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].Title != "ann" || list[1].Title != "bob" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestGroupGet_foundWithAndWithoutParent_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rootID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: rootID, DomainID: domainID, Title: "root"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "child", ParentGroupID: &rootID}); err != nil {
		t.Fatal(err)
	}
	gRoot, err := s.GroupGet(ctx, domainID, rootID)
	if err != nil {
		t.Fatal(err)
	}
	if gRoot.ParentGroupID != nil {
		t.Fatalf("root should have nil parent, got %+v", gRoot.ParentGroupID)
	}
	gChild, err := s.GroupGet(ctx, domainID, childID)
	if err != nil {
		t.Fatal(err)
	}
	if gChild.ParentGroupID == nil || *gChild.ParentGroupID != rootID {
		t.Fatalf("want parent %s, got %+v", rootID, gChild.ParentGroupID)
	}
	_, err = s.GroupGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGroupList_emptyWithItemsIncludingParent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.GroupList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
	parentID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "P"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "C", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	list, err = s.GroupList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	// ORDER BY title: C before P
	if list[0].ID != childID || list[1].ID != parentID {
		t.Fatalf("unexpected order or ids: %+v", list)
	}
	if list[0].ParentGroupID == nil || *list[0].ParentGroupID != parentID {
		t.Fatalf("child list row: want parent %s, got %+v", parentID, list[0].ParentGroupID)
	}
	if list[1].ParentGroupID != nil {
		t.Fatalf("parent row should have nil ParentGroupID, got %+v", list[1].ParentGroupID)
	}
}

func TestGroupSetParent(t *testing.T) {
	ctx := context.Background()

	t.Run("setParentSuccess", func(t *testing.T) {
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
		if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "chi"}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupSetParent(ctx, domainID, childID, &parentID); err != nil {
			t.Fatal(err)
		}
		g, err := s.GroupGet(ctx, domainID, childID)
		if err != nil {
			t.Fatal(err)
		}
		if g.ParentGroupID == nil || *g.ParentGroupID != parentID {
			t.Fatalf("want parent %s, got %+v", parentID, g.ParentGroupID)
		}
	})

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
			t.Fatalf("want nil parent, got %+v", g.ParentGroupID)
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
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
		if !strings.Contains(err.Error(), "own parent") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		g1 := uuid.NewString()
		g2 := uuid.NewString()
		g3 := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: g1, DomainID: domainID, Title: "g1"}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupCreate(ctx, &store.Group{ID: g2, DomainID: domainID, Title: "g2", ParentGroupID: &g1}); err != nil {
			t.Fatal(err)
		}
		if err := s.GroupCreate(ctx, &store.Group{ID: g3, DomainID: domainID, Title: "g3", ParentGroupID: &g2}); err != nil {
			t.Fatal(err)
		}
		// g1 -> g2 -> g3; setting g1's parent to g3 closes the cycle.
		err := s.GroupSetParent(ctx, domainID, g1, &g3)
		if !errors.Is(err, store.ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nonExistentGroup", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		parentID := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "p"}); err != nil {
			t.Fatal(err)
		}
		err := s.GroupSetParent(ctx, domainID, uuid.NewString(), &parentID)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("nonExistentParent", func(t *testing.T) {
		s := newTestStore(t)
		domainID := uuid.NewString()
		if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
			t.Fatal(err)
		}
		childID := uuid.NewString()
		if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "c"}); err != nil {
			t.Fatal(err)
		}
		fakeParent := uuid.NewString()
		err := s.GroupSetParent(ctx, domainID, childID, &fakeParent)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestResourceGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
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
	_, err = s.ResourceGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestResourceList_emptyAndWithItems(t *testing.T) {
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
	list, err := s.ResourceList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: uuid.NewString(), DomainID: other, Title: "x"}); err != nil {
		t.Fatal(err)
	}
	list, err = s.ResourceList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].Title != "a" || list[1].Title != "z" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestAccessTypeCreateAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.AccessTypeList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
	a1 := uuid.NewString()
	a2 := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a1, DomainID: domainID, Title: "write", Bit: 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a2, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	list, err = s.AccessTypeList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	// ORDER BY bit: read(1) then write(4)
	if list[0].Bit != 1 || list[0].ID != a2 || list[1].Bit != 4 || list[1].ID != a1 {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestPermissionGet_foundAndNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rid := uuid.NewString()
	pid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
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
	_, err = s.PermissionGet(ctx, domainID, uuid.NewString())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPermissionList_emptyAndWithItems(t *testing.T) {
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
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.PermissionList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("want empty, got %d", len(list))
	}
	p1 := uuid.NewString()
	p2 := uuid.NewString()
	roid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: roid, DomainID: other, Title: "r2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: p1, DomainID: domainID, Title: "zebra", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: p2, DomainID: domainID, Title: "apple", ResourceID: rid, AccessMask: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: other, Title: "other", ResourceID: roid, AccessMask: 3}); err != nil {
		t.Fatal(err)
	}
	list, err = s.PermissionList(ctx, domainID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].Title != "apple" || list[1].Title != "zebra" {
		t.Fatalf("order by title: got %+v", list)
	}
}

func TestAddUserToGroup_fkViolation(t *testing.T) {
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

func TestGrantUserPermission_fkViolation(t *testing.T) {
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

func TestGrantGroupPermission_fkViolation(t *testing.T) {
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

func TestRemoveUserFromGroup_successAndNotFound(t *testing.T) {
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
	if err := s.RemoveUserFromGroup(ctx, domainID, uid, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestRevokeUserPermission_successAndNotFound(t *testing.T) {
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

func TestRevokeGroupPermission_successAndNotFound(t *testing.T) {
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
