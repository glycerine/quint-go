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

func BenchmarkTestCommandPreparation(b *testing.B) {
	var library strings.Builder
	library.WriteString("module Shared { pure def identity(x)=x ")
	for i := range 24 {
		fmt.Fprintf(&library, "pure def plus%d(x)=identity(x)+%d ", i, i)
	}
	library.WriteString("} module A { const N:int import Shared.* val firstTest=identity(N)==N val secondTest=plus23(N)==N+23 } module M { import A(N=1) as I import A(N=2) as J }")
	for _, tc := range []struct{ name, source, main string }{
		{"small", "module M { val exampleTest=true }", "M"},
		{"imports_and_instances", library.String(), "M"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			input := filepath.Join(b.TempDir(), "model.qnt")
			if err := os.WriteFile(input, []byte(tc.source), 0600); err != nil {
				b.Fatal(err)
			}
			args := []string{input, "--main=" + tc.main, "--seed=0", "--max-samples=1", "--verbosity=0"}
			b.ReportAllocs()
			for b.Loop() {
				if status := commandTest(context.Background(), args, io.Discard, io.Discard); status != 0 {
					b.Fatal("test command failed", status)
				}
			}
		})
	}
}
