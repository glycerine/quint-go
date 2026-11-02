package quint

import "testing"

func TestResolveLocalModuleForwardReferences(t *testing.T) {
	ms, err := ParseModules(`module M {
 val use: T = f(value)
 def f(x: T): T = x
 const value: T
 type T = int
 assume valid = use == value
 }`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolveLocalModule(ms[0])
	if err != nil || len(r.Errors) != 0 {
		t.Fatalf("resolution %v %+v", err, r)
	}
	ds := ms[0].Declarations
	for _, tc := range []struct{ reference, definition ID }{{ds[0].Expr.ID, ds[1].ID}, {ds[0].Expr.Args[0].ID, ds[2].ID}, {*ds[0].TypeAnnotation.ID, ds[3].ID}, {ds[1].Expr.Body.ID, ds[1].Expr.Params[0].ID}, {ds[0].ID, ds[0].ID}} {
		if got := r.Table.Get(tc.reference); got.ID != tc.definition {
			t.Fatalf("lookup %s: %+v", tc.reference, got)
		}
	}
	if r.Table.Get(ds[0].ID).Declaration.TypeAnnotation == nil {
		t.Fatal("definition self-entry lost annotation")
	}
}
func TestResolveLocalModuleDiagnostics(t *testing.T) {
	ms, err := ParseModules(`module M { val x = missing val f = (a,a) => a var y: Missing }`)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolveLocalModule(ms[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 4 || r.Errors[0].Code != QNT404 || r.Errors[1].Code != QNT101 || r.Errors[2].Code != QNT101 || r.Errors[3].Message != "Type alias 'Missing' not found" {
		t.Fatalf("errors %+v", r.Errors)
	}
	for _, directive := range []string{"import A", "export A", "import A(N=1) as B"} {
		ms, err := ParseModules("module M { " + directive + " }")
		if err != nil {
			t.Fatal(err)
		}
		if r, err := ResolveLocalModule(ms[0]); err == nil || r != nil {
			t.Fatal("silently resolved unlinked module")
		}
	}
}
