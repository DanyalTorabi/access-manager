package api

import (
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

func TestUserAuthzResourceDTOs_nilInput(t *testing.T) {
	got := userAuthzResourceDTOs(nil)
	if got == nil {
		t.Fatal("expected non-nil slice for nil input, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got len %d", len(got))
	}
}

func TestUserAuthzResourceDTOs_emptyInput(t *testing.T) {
	got := userAuthzResourceDTOs([]store.UserAuthzResource{})
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got len %d", len(got))
	}
}

func TestUserAuthzResourceDTOs_fieldMapping(t *testing.T) {
	input := []store.UserAuthzResource{
		{ResourceID: "res-1", EffectiveMask: 5},
		{ResourceID: "res-2", EffectiveMask: 0},
	}
	got := userAuthzResourceDTOs(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].ResourceID != "res-1" {
		t.Errorf("item[0].ResourceID: want %q, got %q", "res-1", got[0].ResourceID)
	}
	if got[0].EffectiveMask != "5" {
		t.Errorf("item[0].EffectiveMask: want %q, got %q", "5", got[0].EffectiveMask)
	}
	if got[1].ResourceID != "res-2" {
		t.Errorf("item[1].ResourceID: want %q, got %q", "res-2", got[1].ResourceID)
	}
	if got[1].EffectiveMask != "0" {
		t.Errorf("item[1].EffectiveMask: want %q, got %q", "0", got[1].EffectiveMask)
	}
}

func TestGroupAuthzResourceDTOs_nilInput(t *testing.T) {
	got := groupAuthzResourceDTOs(nil)
	if got == nil {
		t.Fatal("expected non-nil slice for nil input, got nil")
	}
}

func TestGroupAuthzResourceDTOs_fieldMapping(t *testing.T) {
	input := []store.GroupAuthzResource{
		{ResourceID: "res-A", Mask: 6},
	}
	got := groupAuthzResourceDTOs(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].ResourceID != "res-A" {
		t.Errorf("ResourceID: want %q, got %q", "res-A", got[0].ResourceID)
	}
	if got[0].Mask != "6" {
		t.Errorf("Mask: want %q, got %q", "6", got[0].Mask)
	}
}

func TestResourceAuthzUserDTOs_nilInput(t *testing.T) {
	got := resourceAuthzUserDTOs(nil)
	if got == nil {
		t.Fatal("expected non-nil slice for nil input, got nil")
	}
}

func TestResourceAuthzUserDTOs_fieldMapping(t *testing.T) {
	input := []store.ResourceAuthzUser{
		{UserID: "user-X", EffectiveMask: 7},
	}
	got := resourceAuthzUserDTOs(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].UserID != "user-X" {
		t.Errorf("UserID: want %q, got %q", "user-X", got[0].UserID)
	}
	if got[0].EffectiveMask != "7" {
		t.Errorf("EffectiveMask: want %q, got %q", "7", got[0].EffectiveMask)
	}
}

func TestResourceAuthzGroupDTOs_nilInput(t *testing.T) {
	got := resourceAuthzGroupDTOs(nil)
	if got == nil {
		t.Fatal("expected non-nil slice for nil input, got nil")
	}
}

func TestResourceAuthzGroupDTOs_fieldMapping(t *testing.T) {
	input := []store.ResourceAuthzGroup{
		{GroupID: "grp-1", Mask: 3},
	}
	got := resourceAuthzGroupDTOs(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].GroupID != "grp-1" {
		t.Errorf("GroupID: want %q, got %q", "grp-1", got[0].GroupID)
	}
	if got[0].Mask != "3" {
		t.Errorf("Mask: want %q, got %q", "3", got[0].Mask)
	}
}
