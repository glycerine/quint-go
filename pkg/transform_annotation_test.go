package quint

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDeclarationApplicationResolution(t *testing.T) {
	ms, err := ParseModules("module M { type Box[a]=List[a] const c:Box[int] var v:Box[str] def f(x:Box[int]):Box[int]=def g(y:Box[int]):Box[int]=y; g(x) assume ok=List(1).forall((x)=>x>0) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	if len(linked.Errors) != 0 {
		t.Fatal(linked.Errors)
	}
	before, _ := json.Marshal(ms)
	r := NewTypeApplicationResolver(linked.Table)
	ds, err := r.ResolveDeclarations(ms[0].Declarations)
	if err != nil {
		t.Fatal(err)
	}
	if ds[1].TypeAnnotation.Kind != "list" || ds[2].TypeAnnotation.Kind != "list" {
		t.Fatal("top-level annotation")
	}
	f := ds[3]
	if f.TypeAnnotation.Res.Kind != "list" || f.Expr.Params[0].TypeAnnotation.Kind != "list" {
		t.Fatal("operator annotation")
	}
	g := f.Expr.Body.Opdef
	if g.TypeAnnotation.Res.Kind != "list" || g.Expr.Params[0].TypeAnnotation.Kind != "list" {
		t.Fatal("local annotation")
	}
	after, _ := json.Marshal(ms)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}

func TestTransformDeclarationTypesCoverage(t *testing.T) {
	ms, err := ParseModules("module A { const F:int=>int } module M { import A(F=(x)=>x) as I assume ok=List(1).forall((x)=>x>0) }")
	if err != nil {
		t.Fatal(err)
	}
	ms[1].Declarations[0].Overrides[0].Expr.Params[0].TypeAnnotation = &Type{Kind: "int"}
	ms[1].Declarations[1].Assumption.Args[1].Params[0].TypeAnnotation = &Type{Kind: "int"}
	calls := 0
	transform := func(t Type) (Type, error) { calls++; return Type{Kind: "str"}, nil }
	for _, d := range ms[1].Declarations {
		out, err := TransformDeclarationTypes(d, transform)
		if err != nil {
			t.Fatal(err)
		}
		if d.Kind == "instance" && out.Overrides[0].Expr.Params[0].TypeAnnotation.Kind != "str" {
			t.Fatal("override parameter")
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
	sentinel := errors.New("stop")
	_, err = TransformDeclarationTypes(ms[1].Declarations[0], func(Type) (Type, error) { return Type{}, sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatal("error not propagated")
	}
}
