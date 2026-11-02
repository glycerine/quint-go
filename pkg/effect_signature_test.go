package quint

import (
	"testing"
)

func TestEffectSignatureDeclarationsAndLookup(t *testing.T) {
	ms, err := ParseModules("module M { const c:int const f:(int)=>int var x:int val a=c val b=x val z=f(c) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(ms)
	if err != nil {
		t.Fatal(err)
	}
	c := NewEffectSignatureContext(linked.Table)
	ds := ms[0].Declarations
	for i, d := range ds {
		if c.SeedDeclaration(d) != (i < 3) {
			t.Fatal(d)
		}
	}
	for i, want := range map[int]string{3: "Pure", 4: "Read['x']"} {
		e, failure := c.EffectForName(ds[i].Expr.Name, ds[i].Expr.ID, 2)
		if failure != nil || EffectString(e) != want {
			t.Fatalf("%s %v", EffectString(e), failure)
		}
	}
	a, e := c.EffectForName("f", ds[5].Expr.ID, 1)
	if e != nil {
		t.Fatal(e)
	}
	b, e := c.EffectForName("f", ds[5].Expr.ID, 1)
	if e != nil {
		t.Fatal(e)
	}
	if EffectString(a) == EffectString(b) || len(a.Params) != 1 {
		t.Fatal("constant operator not freshened")
	}
	state, _ := c.Fetch(ds[2].ID)
	if state.Effect.Components[0].Entity.StateVariables[0].Reference != ds[2].ID {
		t.Fatal("state reference")
	}
	if len(linked.Errors) != 0 {
		t.Fatal(linked.Errors)
	}
}

func TestEffectSignatureInstancesAndErrors(t *testing.T) {
	id, _ := ParseID("1")
	ref, _ := ParseID("2")
	table := NewExpressionNameResolver(nil).Table
	def := LookupDefinition{ID: id, ImportedFrom: &Declaration{Kind: "instance"}, Namespaces: []string{"Inner", "Outer"}}
	table.Set(ref, def)
	c := NewEffectSignatureContext(table)
	c.Location = "checking"
	c.SeedDeclaration(Declaration{Kind: "var", ID: id, Name: "x"})
	got, e := c.EffectForName("x", ref, 0)
	if e != nil || EffectString(got) != "Read['Outer::Inner::x']" {
		t.Fatalf("%+v %v", got, e)
	}
	source, _ := c.Fetch(id)
	if EffectString(source.Effect) != "Read['x']" {
		t.Fatal("mutated saved effect")
	}
	def.ImportedFrom = &Declaration{Kind: "import"}
	table.Set(ref, def)
	got, e = c.EffectForName("x", ref, 0)
	if e != nil || EffectString(got) != "Read['x']" {
		t.Fatal(got, e)
	}
	previous := ErrorLeaf("earlier", "failed")
	c.Save(id, EffectScheme{}, &previous)
	if _, e = c.EffectForName("x", ref, 0); e == nil || e.Location != "checking" || len(e.Children) != 1 {
		t.Fatal(e)
	}
	missing, _ := ParseID("3")
	if _, e = c.EffectForName("missing", missing, 0); e == nil || *e.Message != "Signature not found for name: missing" {
		t.Fatal(e)
	}
	if _, e = c.Fetch(missing); e == nil || *e.Message != "Couldn't find any result for id 3 while checking" {
		t.Fatal(e)
	}
	// Builtins take precedence over lookup failures and retain fresh variables.
	a, e := c.EffectForName("iadd", ref, 2)
	if e != nil {
		t.Fatal(e)
	}
	b, e := c.EffectForName("iadd", ref, 2)
	if e != nil || EffectString(a) == EffectString(b) {
		t.Fatal("builtin not freshened")
	}
}
