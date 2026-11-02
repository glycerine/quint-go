package quint

import "testing"

func TestLinkedModuleReferences(t *testing.T) {
	ms, err := ParseModules(`module A { const N: int val x = N type T = int }
 module B { import A as Alias export Alias.* val y: Alias::T = Alias::x }
 module C { import B.* val z = x }`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 0 {
		t.Fatalf("link %v %+v", err, r.Errors)
	}
	b := ms[1].Declarations[2]
	c := ms[2].Declarations[1]
	if r.Table.Get(b.Expr.ID).ID != ms[0].Declarations[1].ID || r.Table.Get(c.Expr.ID).ID != ms[0].Declarations[1].ID {
		t.Fatal("linked definition identity")
	}
	if r.Table.Get(*b.TypeAnnotation.ID).ID != ms[0].Declarations[2].ID {
		t.Fatal("qualified type")
	}
	if r.Modules.Get("Alias") == nil {
		t.Fatal("alias not registered")
	}
}
func TestLinkVisibilityAndSourceOrder(t *testing.T) {
	ms, _ := ParseModules(`module A { val x = 1 } module B { import A.* } module C { import B.* val y = x }`)
	r, err := ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 1 || r.Errors[0].Code != QNT404 {
		t.Fatalf("hidden import %+v %v", r.Errors, err)
	}
	ms, _ = ParseModules(`module A { val x = 1 } module B { import A.x val x = 2 }`)
	r, err = ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 2 || *r.Errors[1].Reference != ms[1].Declarations[1].ID {
		t.Fatalf("source-order conflict %+v", r.Errors)
	}
	ms, _ = ParseModules(`module A { import Missing.* val x = missing }`)
	r, err = ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 2 || r.Errors[0].Code != QNT405 || r.Errors[1].Code != QNT404 {
		t.Fatal("collection/reference diagnostics")
	}

}
