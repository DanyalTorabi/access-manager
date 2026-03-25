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
}
