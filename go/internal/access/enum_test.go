package access

import (
	"errors"
	"testing"

	"github.com/dtorabi/access-manager/internal/store"
)

// accessTypeEntry is a (title, bit) pair used by makeTypes.
type accessTypeEntry struct {
	title string
	bit   uint64
}

// makeTypes builds a []store.AccessType from (title, bit) pairs.
func makeTypes(entries ...accessTypeEntry) []store.AccessType {
	out := make([]store.AccessType, 0, len(entries))
	for _, e := range entries {
		out = append(out, store.AccessType{
			ID:       "id",
			DomainID: "dom",
			Title:    e.title,
			Bit:      e.bit,
		})
	}
	return out
}

func TestMaskToTitles(t *testing.T) {
	types := makeTypes(
		accessTypeEntry{"read", uint64(1)},
		accessTypeEntry{"write", uint64(2)},
		accessTypeEntry{"delete", uint64(4)},
	)

	tests := []struct {
		name  string
		mask  uint64
		types []store.AccessType
		want  []string
	}{
		{
			name:  "zero mask returns empty slice",
			mask:  0,
			types: types,
			want:  []string{},
		},
		{
			name:  "single title",
			mask:  1,
			types: types,
			want:  []string{"read"},
		},
		{
			name:  "multiple titles sorted",
			mask:  7,
			types: types,
			want:  []string{"delete", "read", "write"},
		},
		{
			name:  "bit with no registered title becomes sentinel",
			mask:  8, // bit value 8, no matching access type
			types: types,
			want:  []string{"_bit:8"},
		},
		{
			name:  "mix of known and unknown bits",
			mask:  1 | 8,
			types: types,
			want:  []string{"_bit:8", "read"},
		},
		{
			name:  "empty types slice produces all sentinels",
			mask:  3,
			types: []store.AccessType{},
			want:  []string{"_bit:1", "_bit:2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskToTitles(tc.mask, tc.types)
			if len(got) != len(tc.want) {
				t.Fatalf("MaskToTitles(%d) = %v, want %v", tc.mask, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("MaskToTitles(%d)[%d] = %q, want %q", tc.mask, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestMaskToTitles_nilTypes(t *testing.T) {
	got := MaskToTitles(0, nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestTitlesToMask(t *testing.T) {
	types := makeTypes(
		accessTypeEntry{"read", uint64(1)},
		accessTypeEntry{"write", uint64(2)},
		accessTypeEntry{"delete", uint64(4)},
	)

	tests := []struct {
		name    string
		titles  []string
		types   []store.AccessType
		want    uint64
		wantErr string
	}{
		{
			name:   "empty titles",
			titles: []string{},
			types:  types,
			want:   0,
		},
		{
			name:   "nil titles",
			titles: nil,
			types:  types,
			want:   0,
		},
		{
			name:   "single title",
			titles: []string{"read"},
			types:  types,
			want:   1,
		},
		{
			name:   "multiple titles",
			titles: []string{"read", "write"},
			types:  types,
			want:   3,
		},
		{
			name:   "all titles",
			titles: []string{"read", "write", "delete"},
			types:  types,
			want:   7,
		},
		{
			name:    "unknown title returns error",
			titles:  []string{"read", "exec"},
			types:   types,
			wantErr: `access: unknown permission title "exec"`,
		},
		{
			name:    "unknown title with empty types",
			titles:  []string{"read"},
			types:   []store.AccessType{},
			wantErr: `access: unknown permission title "read"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TitlesToMask(tc.titles, tc.types)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
				}
				var ute *UnknownTitleError
				if !errors.As(err, &ute) {
					t.Errorf("error is not *UnknownTitleError: %T", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("TitlesToMask = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAllocateNextBit(t *testing.T) {
	tests := []struct {
		name    string
		types   []store.AccessType
		want    uint64
		wantErr error
	}{
		{
			name:  "empty domain: first available bit is 1",
			types: []store.AccessType{},
			want:  1,
		},
		{
			name:  "bit 1 taken: returns 2",
			types: makeTypes(accessTypeEntry{"read", uint64(1)}),
			want:  2,
		},
		{
			name:  "bits 1 and 2 taken: returns 4",
			types: makeTypes(accessTypeEntry{"read", uint64(1)}, accessTypeEntry{"write", uint64(2)}),
			want:  4,
		},
		{
			name:  "sparse gap: bits 1 and 4 taken, returns 2",
			types: makeTypes(accessTypeEntry{"read", uint64(1)}, accessTypeEntry{"delete", uint64(4)}),
			want:  2,
		},
		{
			name:    "all 63 bits taken: ErrBitsExhausted",
			types:   allBitsTypes(),
			wantErr: ErrBitsExhausted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AllocateNextBit(tc.types)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("AllocateNextBit = %d, want %d", got, tc.want)
			}
		})
	}
}

// allBitsTypes builds a slice of 63 access types occupying every available bit.
func allBitsTypes() []store.AccessType {
	out := make([]store.AccessType, 63)
	for pos := uint64(0); pos <= 62; pos++ {
		out[pos] = store.AccessType{
			ID:       "id",
			DomainID: "dom",
			Title:    "type",
			Bit:      uint64(1) << pos,
		}
	}
	return out
}
