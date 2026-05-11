package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.DomainList(ctx, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
	}
	d1 := uuid.NewString()
	d2 := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: d1, Title: "zebra"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: d2, Title: "apple"}); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.DomainList(ctx, allOpts)
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


func TestDomainDelete_emptyDomain(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DomainDelete(ctx, domainID); err != nil {
		t.Fatal(err)
	}
	_, err := s.DomainGet(ctx, domainID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}


func TestDomainCreate_duplicateID_conflict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "a"}); err != nil {
		t.Fatal(err)
	}
	err := s.DomainCreate(ctx, &store.Domain{ID: id, Title: "b"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}


func TestDomainList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Offset: 0, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("total: want 5, got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("items: want 2, got %d", len(list))
	}
	if list[0].Title != "a" || list[1].Title != "b" {
		t.Fatalf("first page: %+v", list)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Offset: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("page 2: items=%d total=%d", len(list), total)
	}
	if list[0].Title != "d" || list[1].Title != "e" {
		t.Fatalf("page 2 content: %+v", list)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Offset: 10, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 0 {
		t.Fatalf("past end: items=%d total=%d", len(list), total)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Offset: 0, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 1 || list[0].Title != "a" {
		t.Fatalf("limit 1: items=%d total=%d list=%+v", len(list), total, list)
	}
}


func TestDomainList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Alpha", "Beta", "Alphabet"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "alph"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "Alpha" || list[1].Title != "Alphabet" {
		t.Fatalf("unexpected titles: %+v", list)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("want 0, got %d items total=%d", len(list), total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: ""})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("empty search should return all, got total=%d", total)
	}
}


func TestDomainList_searchWithPagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		title := "test-" + string(rune('a'+i))
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: "other"}); err != nil {
		t.Fatal(err)
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 2, Offset: 0, Search: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("total should be 5 (all matching), got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("page size should be 2, got %d", len(list))
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 2, Offset: 4, Search: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 1 {
		t.Fatalf("last page: want total=5 items=1, got total=%d items=%d", total, len(list))
	}
}


func TestDomainList_searchEscapesWildcards(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"100% done", "normal", "test_case"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "%"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("search for literal %%: want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "100% done" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "_"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("search for literal _: want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "test_case" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: `\`})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("search for literal backslash: want 0, got %d items total=%d", len(list), total)
	}
}


func TestDomainList_searchType(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Alpha", "Alphabet", "Beta"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "Alpha", SearchType: store.SearchStartsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("starts_with Alpha: want 2, got %d items total=%d", len(list), total)
	}

	list, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "bet", SearchType: store.SearchEndsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("ends_with bet: want 1, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "Alphabet" {
		t.Fatalf("unexpected title: %s", list[0].Title)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "lph", SearchType: store.SearchContains})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("contains lph: want 2, got total=%d", total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "lph"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("empty SearchType (default contains) lph: want 2, got total=%d", total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "Alp", SearchType: store.SearchEndsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("ends_with Alp: want 0, got total=%d", total)
	}

	_, total, err = s.DomainList(ctx, store.ListOpts{Limit: 100, Search: "eta", SearchType: store.SearchStartsWith})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("starts_with eta: want 0, got total=%d", total)
	}
}


func TestDomainList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Alpha", "Beta", "Charlie"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3, got %d", len(list))
	}
	if list[0].Title != "Charlie" || list[2].Title != "Alpha" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}


func TestDomainList_sortAscExplicit(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, title := range []string{"Charlie", "Alpha", "Beta"} {
		if err := s.DomainCreate(ctx, &store.Domain{ID: uuid.NewString(), Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.DomainList(ctx, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderAsc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Alpha" || list[2].Title != "Charlie" {
		t.Fatalf("asc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

