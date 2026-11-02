package quint

import "testing"

func TestFlattenDependencies(t *testing.T) {
	ms, err := ParseModules("module A { const c:int def f(x)=x+c } module B { import A as I val a=I::f(1)+I::f(2) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	got, err := DependentDefinitions(*ms[1].Declarations[1].Expr, linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "I::c" || got[1].Name != "I::f" {
		t.Fatal(got)
	}
	if ms[0].Declarations[1].Name != "f" {
		t.Fatal("input changed")
	}
}
func TestFlattenSkipsLocalAndInstance(t *testing.T) {
	ms, err := ParseModules("module A { const c:int val a=c } module B { import A(c=1) as I val b=def local(x)=x; local(I::a) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	got, err := DependentDefinitions(*ms[1].Declarations[1].Expr, linked.Table)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
