package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidateSort(t *testing.T) {
	multi := []string{"title", "resource_id"}
	single := []string{"title"}

	tests := []struct {
		name       string
		sort       string
		allowed    []string
		want       string
		wantErr    bool
		errContain string
	}{
		{"empty defaults to first", "", multi, "title", false, ""},
		{"valid first field", "title", multi, "title", false, ""},
		{"valid second field", "resource_id", multi, "resource_id", false, ""},
		{"unknown field", "unknown", multi, "", true, "sort must be one of"},
		{"single allowed empty", "", single, "title", false, ""},
		{"single allowed valid", "title", single, "title", false, ""},
		{"single allowed invalid", "other", single, "", true, "sort must be one of"},
		{"nil allowed", "", nil, "", true, "no sortable fields defined"},
		{"empty allowed slice", "title", []string{}, "", true, "no sortable fields defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateSort(tt.sort, tt.allowed)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error should contain %q, got: %v", tt.errContain, err)
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

func TestSanitizeListOpts(t *testing.T) {
	t.Run("order defaults to asc", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{Limit: 10})
		if opts.Order != OrderAsc {
			t.Fatalf("Order: got %q, want %q", opts.Order, OrderAsc)
		}
	})

	t.Run("order preserved when set", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{Limit: 10, Order: OrderDesc})
		if opts.Order != OrderDesc {
			t.Fatalf("Order: got %q, want %q", opts.Order, OrderDesc)
		}
	})

	t.Run("limit defaults when zero", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{})
		if opts.Limit != DefaultLimit {
			t.Fatalf("Limit: got %d, want %d", opts.Limit, DefaultLimit)
		}
	})

	t.Run("limit defaults when negative", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{Limit: -5})
		if opts.Limit != DefaultLimit {
			t.Fatalf("Limit: got %d, want %d", opts.Limit, DefaultLimit)
		}
	})

	t.Run("limit capped at max", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{Limit: MaxLimit + 50})
		if opts.Limit != MaxLimit {
			t.Fatalf("Limit: got %d, want %d", opts.Limit, MaxLimit)
		}
	})

	t.Run("negative offset floored to zero", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{Limit: 10, Offset: -3})
		if opts.Offset != 0 {
			t.Fatalf("Offset: got %d, want 0", opts.Offset)
		}
	})

	t.Run("valid values unchanged", func(t *testing.T) {
		opts := SanitizeListOpts(ListOpts{Limit: 50, Offset: 10, Order: OrderDesc})
		if opts.Limit != 50 || opts.Offset != 10 || opts.Order != OrderDesc {
			t.Fatalf("got Limit=%d Offset=%d Order=%q", opts.Limit, opts.Offset, opts.Order)
		}
	})
}

func TestInvalidInputError(t *testing.T) {
	t.Run("Error_includesDetail", func(t *testing.T) {
		err := NewInvalidInput("empty patch")
		if got, want := err.Error(), "store: invalid input: empty patch"; got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})
	t.Run("Error_emptyDetail", func(t *testing.T) {
		err := NewInvalidInput("")
		if got, want := err.Error(), "store: invalid input"; got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})
	t.Run("ErrorsIs_invalidInput", func(t *testing.T) {
		err := NewInvalidInput("anything")
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatal("errors.Is(err, ErrInvalidInput) = false, want true")
		}
	})
	t.Run("ErrorsIs_throughWrap", func(t *testing.T) {
		err := fmt.Errorf("ctx: %w", NewInvalidInput("cycle detected"))
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatal("errors.Is failed through fmt.Errorf wrap")
		}
		var iie *InvalidInputError
		if !errors.As(err, &iie) {
			t.Fatal("errors.As failed through fmt.Errorf wrap")
		}
		if iie.Detail != "cycle detected" {
			t.Fatalf("Detail = %q", iie.Detail)
		}
	})
}
