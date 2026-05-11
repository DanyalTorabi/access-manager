package sqlite

import (
	"context"
	"errors"
	"testing"
	"strings"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
	allOpts := store.GroupListOpts{ListOpts: store.ListOpts{Offset: 0, Limit: 100}}
	list, total, err := s.GroupList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	parentID := uuid.NewString()
	childID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "P"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: childID, DomainID: domainID, Title: "C", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.GroupList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
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


func TestGroupPatch_titleAndParent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	pID := uuid.NewString()
	cID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: pID, DomainID: domainID, Title: "par"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: cID, DomainID: domainID, Title: "chi"}); err != nil {
		t.Fatal(err)
	}
	nt := "child"
	g, err := s.GroupPatch(ctx, domainID, cID, store.GroupPatchParams{Title: &nt, UpdateParent: true, ParentGroupID: &pID})
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "child" || g.ParentGroupID == nil || *g.ParentGroupID != pID {
		t.Fatalf("group: %+v", g)
	}
	g, err = s.GroupPatch(ctx, domainID, cID, store.GroupPatchParams{UpdateParent: true, ParentGroupID: nil})
	if err != nil || g.ParentGroupID != nil {
		t.Fatalf("clear parent: %+v err=%v", g, err)
	}
}


func TestGroupPatch_titleOnly(t *testing.T) {
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
	nt := "renamed"
	g, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{Title: &nt})
	if err != nil || g.Title != "renamed" {
		t.Fatalf("got %+v err=%v", g, err)
	}
}


func TestGroupList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Offset: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}
	if list[0].Title != "b" || list[1].Title != "c" {
		t.Fatalf("content: %+v", list)
	}

	list, total, err = s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Offset: 10, Limit: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 0 {
		t.Fatalf("past end: items=%d total=%d", len(list), total)
	}
}


func TestGroupPatch_parentOnlyError(t *testing.T) {
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
	badParent := uuid.NewString()
	_, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{
		UpdateParent:  true,
		ParentGroupID: &badParent,
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound for non-existent parent, got %v", err)
	}
}


func TestGroupList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Admins", "Editors", "Admin-sub"} {
		if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts: store.ListOpts{Limit: 100, Search: "admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}


func TestGroupList_filterByParentGroupID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	parentID := uuid.NewString()
	child1 := uuid.NewString()
	child2 := uuid.NewString()
	root2 := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "parent"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: child1, DomainID: domainID, Title: "child1", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: child2, DomainID: domainID, Title: "child2", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: root2, DomainID: domainID, Title: "root2"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts:      store.ListOpts{Limit: 100},
		ParentGroupID: &parentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2 children, got %d items total=%d", len(list), total)
	}
	if list[0].ID != child1 || list[1].ID != child2 {
		t.Fatalf("unexpected children: %+v", list)
	}

	_, total, err = s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts: store.ListOpts{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("no filter should return all 4, got total=%d", total)
	}
}


func TestGroupList_searchAndParentCombined(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	parentID := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: parentID, DomainID: domainID, Title: "root"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: "dev-team", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: "ops-team", ParentGroupID: &parentID}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.GroupList(ctx, domainID, store.GroupListOpts{
		ListOpts:      store.ListOpts{Limit: 100, Search: "dev"},
		ParentGroupID: &parentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "dev-team" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}
}


func TestGroupList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Admins", "Editors", "Viewers"} {
		if err := s.GroupCreate(ctx, &store.Group{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.GroupList(ctx, domainID, store.GroupListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc}})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Viewers" || list[2].Title != "Admins" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

