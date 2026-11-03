package quint

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Compare both public constructors against the same checked source graph and
// continuing ID generator. Source parsing and its first analysis belong to the
// caller in both cases; only duplicate preparation is inside the timed loop.
func BenchmarkProjectEvaluatorAnalyzedPreparation(b *testing.B) {
	var library strings.Builder
	library.WriteString("module Shared { pure def identity(x)=x ")
	for i := range 24 {
		fmt.Fprintf(&library, "pure def plus%d(x)=identity(x)+%d ", i, i)
	}
	library.WriteString("} module A { const N:int import Shared.* var x:int action init=x'=N action step=x'=plus23(x) val safe=x>=N } module M { import A(N=1) as I import A(N=2) as J action init=all { I::init, J::init } action step=all { I::step, J::step } val safe=I::safe and J::safe }")
	for _, tc := range []struct{ name, source string }{
		{"small", "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }"},
		{"imports_and_instances", library.String()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			previous, _, ids := analyzedProjectFixture(b, tc.source, nil)
			init, step := "init", "step"
			additions, err := SynthesizeCompilationEntries(CompilationEntries{Target: "json", Init: &init, Step: &step, Invariants: []string{"safe"}}, ids)
			if err != nil {
				b.Fatal(err)
			}
			for i := range additions {
				if additions[i].Name == "q::init" || additions[i].Name == "q::step" {
					additions[i].Qualifier = Run
				}
			}
			working := analyzedProjectModules(previous, additions, "M")
			b.Run("full", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := NewProjectEvaluator(context.Background(), working, ids, "M", nil, false); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("from_analysis", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, additions, ids, "M", nil, false); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
