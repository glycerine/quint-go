package quint

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkRuntimeCollation(b *testing.B) {
	app := normalizePreallocApp
	texts := []string{"z", "Z", "a", "A", "é", "e\u0301", "ß", "ss", "İ", "ı", "😀", "\ue000", "\uffff", "a-b", "a b", "e\u0300"}
	strings := make([]Expr, len(texts))
	nested := make([]Expr, 8)
	for i, text := range texts {
		strings[i] = Expr{Kind: "str", Str: text}
	}
	for i := range nested {
		children := make([]Expr, 8)
		for j := range children {
			children[j] = Expr{Kind: "str", Str: fmt.Sprint(i) + texts[j]}
		}
		nested[i] = app("Set", children...)
	}
	for _, tc := range []struct {
		name  string
		input Expr
	}{
		{"set16", app("Set", strings...)},
		{"nested_sets", app("Set", nested...)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("normalize", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := NormalizeValue(tc.input); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("construct", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := NewRuntimeValue(tc.input); err != nil {
						b.Fatal(err)
					}
				}
			})
			value, err := NewRuntimeValue(tc.input)
			if err != nil {
				b.Fatal(err)
			}
			b.Run("serialize", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := SerializeRuntimeValueContext(context.Background(), value, nil); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
