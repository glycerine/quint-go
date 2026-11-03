package quint

import (
	"fmt"
	"testing"
)

func BenchmarkRuntimeReferenceIterator(b *testing.B) {
	history := runtimeIteratorHistory(b)
	for _, count := range []int{0, 1, 8, 9, 32, 65, 78} {
		mapping := history[count]
		for _, cached := range []bool{false, true} {
			b.Run(fmt.Sprintf("%d/iterator=%v", count, cached), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					seen := 0
					if cached {
						for range mapping.all() {
							seen++
						}
					} else {
						for range mapping.Entries() {
							seen++
						}
					}
					if seen != count {
						b.Fatal("unexpected traversal count")
					}
				}
			})
		}
	}
}

func BenchmarkRuntimeIteratorAssemble(b *testing.B) {
	history := runtimeIteratorHistory(b)
	for _, op := range []string{"Set", "Map"} {
		for _, count := range []int{0, 8, 32, 78} {
			mapping := history[count]
			for _, optimized := range []bool{false, true} {
				b.Run(fmt.Sprintf("%s/%d/iterator=%v", op, count, optimized), func(b *testing.B) {
					assemble := runtimeIteratorAssembleOracle
					if optimized {
						assemble = runtimeWithEntries
					}
					b.ReportAllocs()
					for b.Loop() {
						if _, err := assemble(op, mapping); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
