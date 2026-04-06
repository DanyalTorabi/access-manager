package store

import (
	"strings"
	"testing"
)

func TestValidateSort(t *testing.T) {
	multi := []string{"title", "resource_id"}
	single := []string{"title"}

	tests := []struct {
		name    string
		sort    string
		allowed []string
		want    string
		wantErr bool
	}{
		{"empty defaults to first", "", multi, "title", false},
		{"valid first field", "title", multi, "title", false},
		{"valid second field", "resource_id", multi, "resource_id", false},
		{"unknown field", "unknown", multi, "", true},
		{"single allowed empty", "", single, "title", false},
		{"single allowed valid", "title", single, "title", false},
		{"single allowed invalid", "other", single, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateSort(tt.sort, tt.allowed)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				if !strings.Contains(err.Error(), "sort must be one of") {
					t.Fatalf("error message: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeListOpts_orderDefault(t *testing.T) {
	opts := SanitizeListOpts(ListOpts{Limit: 10})
	if opts.Order != OrderAsc {
		t.Fatalf("Order: got %q, want %q", opts.Order, OrderAsc)
	}

	opts = SanitizeListOpts(ListOpts{Limit: 10, Order: OrderDesc})
	if opts.Order != OrderDesc {
		t.Fatalf("Order preserved: got %q, want %q", opts.Order, OrderDesc)
	}
}
