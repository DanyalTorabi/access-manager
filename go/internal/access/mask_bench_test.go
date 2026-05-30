package access

import (
	"strconv"
	"testing"
)

func BenchmarkCombineMasks(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000} {
		masks := make([]uint64, n)
		for i := range masks {
			masks[i] = uint64(i + 1)
		}
		b.Run("N="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = CombineMasks(masks)
			}
		})
	}
}

