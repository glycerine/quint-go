package quint

import (
	"fmt"
	"testing"
)

func BenchmarkNormalizePreallocation(b *testing.B) {
	integer, app := normalizePreallocInt, normalizePreallocApp
	list := make([]Expr, 128)
	record := make([]Expr, 0, 32)
	pairs := make([]Expr, 16)
	duplicates := make([]Expr, 256)
	for i := range list {
		list[i] = integer(i)
	}
	for i := range pairs {
		record = append(record, Expr{Kind: "str", Str: fmt.Sprint(16 - i)}, app("List", list[i:i+8]...))
		pairs[i] = app("Tup", integer(16-i), app("List", list[i:i+8]...))
	}
	for i := range duplicates {
		duplicates[i] = integer(1)
	}
	for _, tc := range []struct {
		name  string
		value Expr
	}{
		{"scalar", integer(1)},
		{"list128", app("List", list...)},
		{"record16", app("Rec", record...)},
		{"map16", app("Map", pairs...)},
		{"duplicateSet256", app("Set", duplicates...)},
		{"nested", app("List", app("Rec", record...), app("Map", pairs...), app("List", list...))},
	} {
		for _, implementation := range []string{"append", "current"} {
			b.Run(tc.name+"/"+implementation, func(b *testing.B) {
				normalize := normalizeValueAppendOracle
				if implementation == "current" {
					normalize = NormalizeValue
				}
				b.ReportAllocs()
				for b.Loop() {
					if _, err := normalize(tc.value); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
