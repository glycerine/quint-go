package quint

import (
	"strings"
	"testing"
)

func TestNamespaceDefinition(t *testing.T) {
	ms, err := ParseModules("module M { def f(x:T):T=def local(y)=y; iadd(local(x),g(x)) }")
	if err != nil {
		t.Fatal(err)
	}
	d := ms[0].Declarations[0]
	got, err := AddNamespaceToDefinition(d, "I", map[string]bool{"iadd": true, "g": true, "f": true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "I::f" || got.Expr.Params[0].Name != "I::x" || got.Expr.Params[0].TypeAnnotation.Name != "I::T" {
		t.Fatal(got)
	}
	text := ExpressionString(*got.Expr)
	if !strings.Contains(text, "I::local") || !strings.Contains(text, "g(I::x)") {
		t.Fatal(text)
	}
	if d.Name != "f" || d.Expr.Params[0].Name != "x" {
		t.Fatal("input mutated")
	}
	again, err := AddNamespaceToDefinition(got, "I", map[string]bool{"iadd": true, "g": true})
	if err != nil || ExpressionString(*again.Expr) != text {
		t.Fatal("not idempotent", err)
	}
}
