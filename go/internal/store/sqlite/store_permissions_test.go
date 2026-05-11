package sqlite

import (
	"context"
	"errors"
	"testing"
	"fmt"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
	allOpts := store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 0, Limit: 100}}
	list, total, err := s.PermissionList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
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
	list, total, err = s.PermissionList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "apple" || list[1].Title != "zebra" {
		t.Fatalf("order by title: got %+v", list)
	}
}


func TestPermissionPatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "b"}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	pt := "perm"
	p, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{Title: &pt})
	if err != nil || p.Title != "perm" {
		t.Fatalf("patch title: %+v err=%v", p, err)
	}
	p, err = s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{ResourceID: &r2})
	if err != nil || p.ResourceID != r2 {
		t.Fatalf("patch resource: %+v err=%v", p, err)
	}
	m := uint64(7)
	p, err = s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{AccessMask: &m})
	if err != nil || p.AccessMask != 7 {
		t.Fatalf("patch mask: %+v err=%v", p, err)
	}
	if err := s.PermissionDelete(ctx, domainID, pid); err != nil {
		t.Fatal(err)
	}
	_, err = s.PermissionGet(ctx, domainID, pid)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}


func TestPermissionList_pagination(t *testing.T) {
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
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.PermissionCreate(ctx, &store.Permission{
			ID: uuid.NewString(), DomainID: domainID, Title: title, ResourceID: rid, AccessMask: uint64(1 << i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}

	list, total, err = s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Offset: 10, Limit: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 0 {
		t.Fatalf("past end: items=%d total=%d", len(list), total)
	}
}


func TestPermissionPatch_maskOnly(t *testing.T) {
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
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	newMask := uint64(0xFF)
	got, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{AccessMask: &newMask})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessMask != 0xFF {
		t.Fatalf("mask: want 0xff, got %#x", got.AccessMask)
	}
	if got.Title != "p" {
		t.Fatalf("title should be unchanged, got %q", got.Title)
	}
}


func TestPermissionPatch_resourceIDOnly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "r1"}); err != nil {
		t.Fatal(err)
	}
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "r2"}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	got, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{ResourceID: &r2})
	if err != nil {
		t.Fatal(err)
	}
	if got.ResourceID != r2 {
		t.Fatalf("resource_id: want %s, got %s", r2, got.ResourceID)
	}
}


func TestPermissionList_search(t *testing.T) {
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
	for _, title := range []string{"can-read", "can-write", "can-read-all"} {
		if err := s.PermissionCreate(ctx, &store.Permission{
			ID: uuid.NewString(), DomainID: domainID, Title: title, ResourceID: rid, AccessMask: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts: store.ListOpts{Limit: 100, Search: "can-read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}


func TestPermissionList_filterByResourceID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "res1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "res2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p1", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p2", ResourceID: r1, AccessMask: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p3", ResourceID: r2, AccessMask: 4}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts:   store.ListOpts{Limit: 100},
		ResourceID: &r1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2 for r1, got %d items total=%d", len(list), total)
	}

	list, total, err = s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts:   store.ListOpts{Limit: 100},
		ResourceID: &r2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1 for r2, got %d items total=%d", len(list), total)
	}
}


func TestPermissionList_searchAndResourceCombined(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	r1 := uuid.NewString()
	r2 := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r1, DomainID: domainID, Title: "res1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: r2, DomainID: domainID, Title: "res2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "read-doc", ResourceID: r1, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "write-doc", ResourceID: r1, AccessMask: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "read-img", ResourceID: r2, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{
		ListOpts:   store.ListOpts{Limit: 100, Search: "read"},
		ResourceID: &r1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "read-doc" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}
}


func TestPermissionList_sortDesc(t *testing.T) {
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
	for _, title := range []string{"perm-a", "perm-b", "perm-c"} {
		if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: title, ResourceID: rid, AccessMask: 1}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc}})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "perm-c" || list[2].Title != "perm-a" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}


func TestPermissionList_sortByResourceID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	rids := []string{uuid.NewString(), uuid.NewString()}
	for i, rid := range rids {
		if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: fmt.Sprintf("r%d", i)}); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p1", ResourceID: rids[1], AccessMask: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.PermissionCreate(ctx, &store.Permission{ID: uuid.NewString(), DomainID: domainID, Title: "p2", ResourceID: rids[0], AccessMask: 2}); err != nil {
		t.Fatal(err)
	}

	list, _, err := s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "resource_id", Order: store.OrderAsc}})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].ResourceID > list[1].ResourceID {
		t.Fatalf("asc resource_id: %s should come before %s", list[0].ResourceID, list[1].ResourceID)
	}

	list, _, err = s.PermissionList(ctx, domainID, store.PermissionListOpts{ListOpts: store.ListOpts{Limit: 100, Sort: "resource_id", Order: store.OrderDesc}})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ResourceID < list[1].ResourceID {
		t.Fatalf("desc resource_id: %s should come after %s", list[0].ResourceID, list[1].ResourceID)
	}
}

