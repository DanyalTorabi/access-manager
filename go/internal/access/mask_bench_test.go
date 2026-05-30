package access

import (
	"testing"
)

func BenchmarkCombineMasks(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000} {
		masks := make([]uint64, n)
		for i := range masks {
			masks[i] = uint64(i + 1)
		}
		b.Run("N="+itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = CombineMasks(masks)
			}
		})
	}
}

// itoa converts a small non-negative integer to its decimal string
// representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
