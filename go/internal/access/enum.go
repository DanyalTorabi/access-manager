package access

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dtorabi/access-manager/internal/store"
)

// ErrBitsExhausted is returned by AllocateNextBit when all 63 available bits
// in the domain are already assigned to an access type.
var ErrBitsExhausted = errors.New("access: all 63 permission bits are exhausted for this domain")

// UnknownTitleError is returned by TitlesToMask when a requested title has no
// corresponding access type in the provided slice.
type UnknownTitleError struct {
	Title string
}

func (e *UnknownTitleError) Error() string {
	return fmt.Sprintf("access: unknown permission title %q", e.Title)
}

// MaskToTitles converts an access mask to the sorted slice of registered
// permission titles from types. Bits that are set in mask but have no
// matching entry in types are represented as "_bit:V" sentinels (where V is
// the stored bit value, e.g. "_bit:4") so that v2 reads never fail on v1-era
// data whose access types were later deleted or are not yet registered.
//
// Only bit positions in the range [0, 62] are scanned. Bit 63 (1<<63) and
// higher are outside the valid permission space and are silently ignored
// (not included as sentinels).
//
// An empty or zero mask returns an empty non-nil slice.
func MaskToTitles(mask uint64, types []store.AccessType) []string {
	titles := make([]string, 0)
	if mask == 0 {
		return titles
	}

	var covered uint64
	for _, t := range types {
		if t.Bit != 0 && mask&t.Bit != 0 {
			titles = append(titles, t.Title)
			covered |= t.Bit
		}
	}

	// Any bits set in mask but not covered by a registered type become sentinels.
	remaining := mask &^ covered
	for pos := uint64(0); pos <= 62; pos++ {
		candidate := uint64(1) << pos
		if remaining&candidate != 0 {
			titles = append(titles, fmt.Sprintf("_bit:%d", candidate))
		}
	}

	sort.Strings(titles)
	return titles
}

// TitlesToMask converts a slice of access-type titles to the combined access
// mask. Each title must be present in the types slice; an unknown title causes
// an *UnknownTitleError to be returned immediately. An empty titles slice
// returns 0.
//
// Access types with Bit=0 are skipped (not included in the mask) and do not
// cause an error. This ensures consistency with MaskToTitles, which also
// excludes bit=0 entries.
func TitlesToMask(titles []string, types []store.AccessType) (uint64, error) {
	if len(titles) == 0 {
		return 0, nil
	}

	byTitle := make(map[string]uint64, len(types))
	for _, t := range types {
		if t.Bit != 0 {
			byTitle[t.Title] = t.Bit
		}
	}

	var mask uint64
	for _, title := range titles {
		bit, ok := byTitle[title]
		if !ok {
			return 0, &UnknownTitleError{Title: title}
		}
		mask |= bit
	}
	return mask, nil
}

// AllocateNextBit returns the lowest power-of-2 bit value in [1, 1<<62] that
// is not already assigned to any access type in the types slice. The return
// value can be stored directly in store.AccessType.Bit.
//
// Returns ErrBitsExhausted if all 63 available slots are occupied.
func AllocateNextBit(types []store.AccessType) (uint64, error) {
	var used uint64
	for _, t := range types {
		used |= t.Bit
	}
	for pos := uint64(0); pos <= 62; pos++ {
		candidate := uint64(1) << pos
		if used&candidate == 0 {
			return candidate, nil
		}
	}
	return 0, ErrBitsExhausted
}
