package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkRunCommandPreparation(b *testing.B) {
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
			input := filepath.Join(b.TempDir(), "model.qnt")
			if err := os.WriteFile(input, []byte(tc.source), 0600); err != nil {
				b.Fatal(err)
			}
			args := []string{input, "--main=M", "--seed=0", "--max-samples=1", "--max-steps=1", "--invariant=safe", "--verbosity=0"}
			b.ReportAllocs()
			for b.Loop() {
				if status := commandRun(context.Background(), args, io.Discard, io.Discard); status != 0 {
					b.Fatal("run command failed", status)
				}
			}
		})
	}
}
