package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
	"github.com/google/uuid"
)

func TestPatchDomainUserResource(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	title := "d2"
	d, err := s.DomainPatch(ctx, domainID, &title)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "d2" {
		t.Fatalf("domain title: %q", d.Title)
	}
	uid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	ut := "alice"
	u, err := s.UserPatch(ctx, domainID, uid, &ut)
	if err != nil {
		t.Fatal(err)
	}
	if u.Title != "alice" {
		t.Fatalf("user title: %q", u.Title)
	}
	rid := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	rt := "doc"
	r, err := s.ResourcePatch(ctx, domainID, rid, &rt)
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "doc" {
		t.Fatalf("resource title: %q", r.Title)
	}
}

func TestDelete_userGroupResource_success(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.NewString()
	gid := uuid.NewString()
	rid := uuid.NewString()
	if err := s.UserCreate(ctx, &store.User{ID: uid, DomainID: domainID, Title: "u"}); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResourceCreate(ctx, &store.Resource{ID: rid, DomainID: domainID, Title: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UserDelete(ctx, domainID, uid); err != nil {
		t.Fatalf("UserDelete: %v", err)
	}
	if err := s.GroupDelete(ctx, domainID, gid); err != nil {
		t.Fatalf("GroupDelete: %v", err)
	}
	if err := s.ResourceDelete(ctx, domainID, rid); err != nil {
		t.Fatalf("ResourceDelete: %v", err)
	}
}

func TestDelete_notFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	domainID := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: domainID, Title: "d"}); err != nil {
		t.Fatal(err)
	}
	missing := uuid.NewString()
	if err := s.DomainDelete(ctx, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DomainDelete: want ErrNotFound, got %v", err)
	}
	if err := s.UserDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("UserDelete: want ErrNotFound, got %v", err)
	}
	if err := s.GroupDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GroupDelete: want ErrNotFound, got %v", err)
	}
	if err := s.ResourceDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ResourceDelete: want ErrNotFound, got %v", err)
	}
	if err := s.AccessTypeDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("AccessTypeDelete: want ErrNotFound, got %v", err)
	}
	if err := s.PermissionDelete(ctx, domainID, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PermissionDelete: want ErrNotFound, got %v", err)
	}
}

func TestPatch_emptyInvalid_notFound(t *testing.T) {
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
	gid := uuid.NewString()
	if err := s.GroupCreate(ctx, &store.Group{ID: gid, DomainID: domainID, Title: "g"}); err != nil {
		t.Fatal(err)
	}
	aid := uuid.NewString()
	if err := s.AccessTypeCreate(ctx, &store.AccessType{ID: aid, DomainID: domainID, Title: "read", Bit: 1}); err != nil {
		t.Fatal(err)
	}
	pid := uuid.NewString()
	if err := s.PermissionCreate(ctx, &store.Permission{ID: pid, DomainID: domainID, Title: "p", ResourceID: rid, AccessMask: 1}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DomainPatch(ctx, domainID, nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("DomainPatch nil: want ErrInvalidInput, got %v", err)
	}
	badDomain := uuid.NewString()
	title := "x"
	if _, err := s.DomainPatch(ctx, badDomain, &title); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("DomainPatch missing domain: %v", err)
	}
	if _, err := s.UserPatch(ctx, domainID, uuid.NewString(), &title); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("UserPatch not found: %v", err)
	}
	if _, err := s.UserPatch(ctx, domainID, uid, nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("UserPatch nil title: %v", err)
	}
	if _, err := s.ResourcePatch(ctx, domainID, uuid.NewString(), &title); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ResourcePatch not found: %v", err)
	}
	if _, err := s.ResourcePatch(ctx, domainID, rid, nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("ResourcePatch nil: %v", err)
	}
	if _, err := s.GroupPatch(ctx, domainID, gid, store.GroupPatchParams{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("GroupPatch empty: %v", err)
	}
	if _, err := s.GroupPatch(ctx, domainID, uuid.NewString(), store.GroupPatchParams{Title: &title}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GroupPatch missing group: %v", err)
	}
	if _, err := s.AccessTypePatch(ctx, domainID, aid, store.AccessTypePatchParams{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("AccessTypePatch empty: %v", err)
	}
	if _, err := s.AccessTypePatch(ctx, domainID, uuid.NewString(), store.AccessTypePatchParams{Title: &title}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("AccessTypePatch not found: %v", err)
	}
	if _, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("PermissionPatch empty: %v", err)
	}
	if _, err := s.PermissionPatch(ctx, domainID, uuid.NewString(), store.PermissionPatchParams{Title: &title}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PermissionPatch not found: %v", err)
	}
	otherDomain := uuid.NewString()
	if err := s.DomainCreate(ctx, &store.Domain{ID: otherDomain, Title: "o"}); err != nil {
		t.Fatal(err)
	}
	otherRes := uuid.NewString()
	if err := s.ResourceCreate(ctx, &store.Resource{ID: otherRes, DomainID: otherDomain, Title: "or"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PermissionPatch(ctx, domainID, pid, store.PermissionPatchParams{ResourceID: &otherRes}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PermissionPatch foreign resource: want ErrNotFound, got %v", err)
	}
}
