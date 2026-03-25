package access

// CombineMasks returns the bitwise OR of all masks (effective permission set).
func CombineMasks(masks []uint64) uint64 {
	var out uint64
	for _, m := range masks {
		out |= m
	}
	return out
}

// HasAccess reports whether effectiveMask includes all bits in requiredMask.
func HasAccess(effectiveMask, requiredMask uint64) bool {
	if requiredMask == 0 {
		return true
	}
	return (effectiveMask & requiredMask) == requiredMask
}

// HasBit reports whether a single access-type bit is set in effectiveMask.
func HasBit(effectiveMask, bit uint64) bool {
	if bit == 0 {
		return false
	}
	return (effectiveMask & bit) != 0
}
