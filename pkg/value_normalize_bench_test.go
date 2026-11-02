package quint

import "testing"

func BenchmarkNormalizeScalar(b *testing.B) {
	value := Expr{Kind: "str", Str: "runtime scalar"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NormalizeValue(value); err != nil {
			b.Fatal(err)
		}
	}
}
