package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

func TestAccessTypeCreateAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.AccessTypeList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	a1 := uuid.NewString()
	a2 := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a1, DomainID: domainID, Title: "write", Bit: 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a2, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.AccessTypeList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	// ORDER BY bit: read(1) then write(4)
	if list[0].Bit != 1 || list[0].ID != a2 || list[1].Bit != 4 || list[1].ID != a1 {
		t.Fatalf("unexpected list: %+v", list)
	}
}


func TestAccessTypeGetPatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	aid := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: aid, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	got, err := s.AccessTypeGet(ctx, domainID, aid)
	if err != nil || got.Title != "read" || got.Bit != 1 {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	nt := "READ"
	a, err := s.AccessTypePatch(ctx, domainID, aid, store.AccessTypePatchParams{Title: &nt})
	if err != nil || a.Title != "READ" || a.Bit != 1 {
		t.Fatalf("patch title: %+v err=%v", a, err)
	}
	b2 := uint64(2)
	a2, err := s.AccessTypePatch(ctx, domainID, aid, store.AccessTypePatchParams{Bit: &b2})
	if err != nil || a2.Bit != 2 {
		t.Fatalf("patch bit: %+v err=%v", a2, err)
	}
	if err := s.AccessTypeDelete(ctx, domainID, aid); err != nil {
		t.Fatal(err)
	}
	_, err = s.AccessTypeGet(ctx, domainID, aid)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}


func TestAccessTypePatch_duplicateBitConflict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	a1 := uuid.NewString()
	a2 := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a1, DomainID: domainID, Title: "a", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: a2, DomainID: domainID, Title: "b", Bit: 2}); err != nil {
		t.Fatal(err)
	}
	b1 := uint64(2)
	_, err := s.AccessTypePatch(ctx, domainID, a1, store.AccessTypePatchParams{Bit: &b1})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}


func TestAccessTypeList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		bit := uint64(1 << i)
		if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: title, Bit: bit}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Offset: 0, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 3 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}

	list, total, err = s.AccessTypeList(ctx, domainID, store.ListOpts{Offset: 4, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 1 {
		t.Fatalf("last page: items=%d total=%d", len(list), total)
	}
}


func TestAccessTypePatch_bitOnly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	atID := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: atID, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	newBit := uint64(2)
	got, err := s.AccessTypePatch(ctx, domainID, atID, store.AccessTypePatchParams{Bit: &newBit})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bit != 2 {
		t.Fatalf("bit: want 2, got %d", got.Bit)
	}
	if got.Title != "read" {
		t.Fatalf("title should be unchanged, got %q", got.Title)
	}
}


func TestAccessTypeList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"read", "write", "readonly"} {
		if err := s.AccessTypeCreate(ctx, &store.AccessType{
			ID: uuid.NewString(), DomainID: domainID, Title: title, Bit: uint64(1 << i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Limit: 100, Search: "read"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}


func TestAccessTypeList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"Read", "Write", "Execute"} {
		if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: uuid.NewString(), DomainID: domainID, Title: title, Bit: uint64(1 << i)}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.AccessTypeList(ctx, domainID, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Write" || list[2].Title != "Execute" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

