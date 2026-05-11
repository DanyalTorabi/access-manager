package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

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
	allOpts := store.ListOpts{Offset: 0, Limit: 100}
	list, total, err := s.UserList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty, got %d items total=%d", len(list), total)
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
	list, total, err = s.UserList(ctx, domainID, allOpts)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
	if list[0].Title != "ann" || list[1].Title != "bob" {
		t.Fatalf("order by title: got %+v", list)
	}
}


func TestUserList_pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.UserList(ctx, domainID, store.ListOpts{Offset: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("items=%d total=%d", len(list), total)
	}
	if list[0].Title != "b" || list[1].Title != "c" {
		t.Fatalf("content: %+v", list)
	}
}


func TestUserList_search(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Alice", "Bob", "Alicia"} {
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, total, err := s.UserList(ctx, domainID, store.ListOpts{Limit: 100, Search: "ali"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("want 2, got %d items total=%d", len(list), total)
	}
}


func TestUserList_sortDesc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Alice", "Bob", "Charlie"} {
		if err := s.UserCreate(ctx, &store.User{ID: uuid.NewString(), DomainID: domainID, Title: title}); err != nil {
			t.Fatal(err)
		}
	}

	list, _, err := s.UserList(ctx, domainID, store.ListOpts{Limit: 100, Sort: "title", Order: store.OrderDesc})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Title != "Charlie" || list[2].Title != "Alice" {
		t.Fatalf("desc order: got %q, %q, %q", list[0].Title, list[1].Title, list[2].Title)
	}
}

