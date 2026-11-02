package quint

import "testing"

func TestDeclarationTypeGeneration(t *testing.T) {
	ms, err := ParseModules("module M { const n:int def identity(x)=x val a=identity(n) val b=identity(true) var v:int val c=v+1 }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	g := NewTypeConstraintGenerator(TypeUnifier{Table: linked.Table})
	for _, d := range ms[0].Declarations {
		if err = g.GenerateDeclaration(d); err != nil {
			t.Fatal(err)
		}
	}
	if g.failed() {
		for _, e := range g.Context.Errors.All() {
			t.Fatal(e)
		}
	}
	for i, want := range map[int]string{0: "int", 2: "int", 3: "bool", 4: "int", 5: "int"} {
		got := TypeString(g.Context.Types.Get(ms[0].Declarations[i].ID.String()).Type)
		if got != want {
			t.Fatalf("%d: %s", i, got)
		}
	}
	scheme := g.Context.Types.Get(ms[0].Declarations[1].ID.String())
	if len(scheme.TypeVariables) != 1 {
		t.Fatal("operator not generalized", scheme)
	}
	count := 0
	for range g.Scope.All() {
		count++
	}
	if count != 2 {
		t.Fatal("temporary scope leaked", count)
	}
}
func TestDeclarationOverrideAndAnnotationFailures(t *testing.T) {
	for _, source := range []string{
		"module A { const n:int } module M { import A(n=true).* }",
		"module M { def f(x:a):a=x+1 }",
		"module M { val x:bool=1 }",
	} {
		ms, err := ParseModules(source)
		if err != nil {
			t.Fatal(err)
		}
		linked, _ := ResolveLinkedModules(ms)
		g := NewTypeConstraintGenerator(TypeUnifier{Table: linked.Table})
		for _, m := range ms {
			for _, d := range m.Declarations {
				if err = g.GenerateDeclaration(d); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !g.failed() {
			t.Fatal("accepted " + source)
		}
	}
}
func TestAssumptionConstraintGeneration(t *testing.T) {
	ms, err := ParseModules("module M { assume axiom=1 }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	g := NewTypeConstraintGenerator(TypeUnifier{Table: linked.Table})
	if err = g.GenerateDeclaration(ms[0].Declarations[0]); err != nil {
		t.Fatal(err)
	}
	if len(g.Constraints) != 1 {
		t.Fatal("assumption not constrained")
	}
	if _, err = g.Solve(); err != nil {
		t.Fatal(err)
	}
	if !g.failed() {
		t.Fatal("nonboolean assumption accepted")
	}
}
