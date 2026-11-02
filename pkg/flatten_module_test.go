package quint

import "testing"

func TestFlattenModuleImports(t *testing.T) {
	ms, err := ParseModules("module A { const c:int def f(x)=x+c val unused=0 } module B { import A as I val use=I::f(1) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	out, err := FlattenModuleImports(ms[1], linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Declarations) != 3 {
		t.Fatal(out)
	}
	for i, name := range []string{"I::c", "I::f", "use"} {
		if out.Declarations[i].Name != name {
			t.Fatal(out)
		}
	}
	if len(ms[1].Declarations) != 2 || ms[1].Declarations[0].Kind != "import" {
		t.Fatal("input mutated")
	}
}
func TestFlattenLocalAndExportGate(t *testing.T) {
	ms, err := ParseModules("module M { val a=1 val b=a+a }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	out, err := FlattenModuleImports(ms[0], linked.Table)
	if err != nil || len(out.Declarations) != 2 {
		t.Fatal(out, err)
	}
	ms[0].Declarations = append(ms[0].Declarations, Declaration{Kind: "export"})
	if _, err = FlattenModuleImports(ms[0], linked.Table); err == nil {
		t.Fatal("unexpanded export discarded")
	}
}
