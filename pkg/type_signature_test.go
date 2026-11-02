package quint

import (
	"github.com/glycerine/insdict"
	"testing"
)

func TestTypeSignatureInstantiation(t *testing.T) {
	c := TypeSignatureContext{Types: insdict.NewDict[string, TypeScheme](), Location: "testing"}
	id, _ := ParseID("1")
	a, e := c.TypeForName("List", id, 2)
	if e != nil {
		t.Fatal(e)
	}
	b, e := c.TypeForName("List", id, 2)
	if e != nil {
		t.Fatal(e)
	}
	if a.Args[0].Name != "_t0" || b.Args[0].Name != "_t1" {
		t.Fatal(a, b)
	}
	if _, e = c.TypeForName("missing", id, 0); e == nil || *e.Message != "Signature not found for name: missing" {
		t.Fatal(e)
	}
}
func TestTypeSignatureLookupPrecedence(t *testing.T) {
	ms, err := ParseModules("module M { const c:int def f(x)=x val y=f(c) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	c := TypeSignatureContext{Unifier: TypeUnifier{Table: linked.Table}, Types: insdict.NewDict[string, TypeScheme](), Errors: insdict.NewDict[string, ErrorTree](), Location: "checking"}
	f := ms[0].Declarations[1]
	typ, _ := ParseType("(a)=>a")
	c.Types.Set(f.ID.String(), TypeScheme{Type: typ, TypeVariables: []string{"a"}})
	c.Types.Set(ms[0].Declarations[0].ID.String(), TypeScheme{Type: Type{Kind: "str"}})
	call := ms[0].Declarations[2].Expr
	out, e := c.TypeForName("f", call.ID, 1)
	if e != nil || out.Args[0].Name != "_t0" {
		t.Fatal(out, e)
	}
	// Constants use the inferred result, not their original annotation.
	out, e = c.TypeForName("c", call.Args[0].ID, 0)
	if e != nil || out.Kind != "str" {
		t.Fatal(out, e)
	}
	prior := ErrorLeaf("earlier", "failed")
	c.Errors.Set(ms[0].Declarations[0].ID.String(), prior)
	_, e = c.TypeForName("c", call.Args[0].ID, 0)
	if e == nil || *e.Message != "failed" {
		t.Fatal(e)
	}
	// An explicit parameter annotation takes precedence over a saved failure.
	param := f.Expr.Params[0]
	def := LookupDefinition{ID: param.ID, Name: param.Name, Parameter: &Parameter{ID: param.ID, Name: param.Name, TypeAnnotation: &Type{Kind: "bool"}}}
	linked.Table.Set(f.Expr.Body.ID, def)
	c.Errors.Set(param.ID.String(), prior)
	out, e = c.TypeForName(param.Name, f.Expr.Body.ID, 0)
	if e != nil || out.Kind != "bool" {
		t.Fatal(out, e)
	}
}
