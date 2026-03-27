package access

import "testing"

func TestCombineMasks(t *testing.T) {
	tests := []struct {
		name  string
		masks []uint64
		want  uint64
	}{
		{"empty", nil, 0},
		{"empty_slice", []uint64{}, 0},
		{"single", []uint64{0x1}, 0x1},
		{"or", []uint64{0x1, 0x2, 0x4}, 0x7},
		{"overlap", []uint64{0x3, 0x5}, 0x7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CombineMasks(tt.masks); got != tt.want {
				t.Fatalf("CombineMasks(%v) = %#x, want %#x", tt.masks, got, tt.want)
			}
		})
	}
}

func TestHasAccess(t *testing.T) {
	if !HasAccess(0x7, 0) {
		t.Fatal("HasAccess with required 0 should be true")
	}
	if !HasAccess(0x7, 0x3) {
		t.Fatal("expected subset bits")
	}
	if HasAccess(0x1, 0x2) {
		t.Fatal("missing bit should deny")
	}
	if HasAccess(0x1, 0x3) {
		t.Fatal("partial overlap should deny when not all required bits set")
	}
	if !HasAccess(0xff, 0xff) {
		t.Fatal("full match")
	}
}

func TestHasBit(t *testing.T) {
	if HasBit(0, 0x1) {
		t.Fatal("zero bit should be false")
	}
	if !HasBit(0x2, 0x2) {
		t.Fatal("expected bit set")
	}
	if HasBit(0x1, 0x2) {
		t.Fatal("wrong bit")
	}
	if HasBit(0x3, 0) {
		t.Fatal("required bit 0 is invalid and should be false")
	}
	if !HasBit(0x3, 0x1) {
		t.Fatal("low bit set in mask")
	}
}
