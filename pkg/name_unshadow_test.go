package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUnshadowNestedBindings(t *testing.T) {
	ms, err := ParseModules("module M { val x=0 def f(x)=val x=x; List(x).map(y=>y+x) val y=val local=1; local }")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 0 {
		t.Fatalf("%v %+v", err, r.Errors)
	}
	before, _ := json.Marshal(ms)
	m := UnshadowNames(ms[0], r.Table)
	lambda := m.Declarations[1].Expr
	wantParam := "x_" + lambda.ID.String()
	if lambda.Params[0].Name != wantParam {
		t.Fatal("lambda parameter")
	}
	let := lambda.Body
	wantLet := "x_" + let.ID.String()
	if let.Opdef.Name != wantLet || let.Opdef.Expr.Name != wantLet {
		t.Fatal("recursive local binding")
	}
	var check func(*Expr)
	check = func(e *Expr) {
		if e == nil {
			return
		}
		if e.Kind == "name" || e.Kind == "app" {
			d := r.Table.Get(e.ID)
			name := e.Name
			if e.Kind == "app" {
				name = e.Opcode
			}
			if d.ID == let.Opdef.ID && (name != wantLet || d.Name != wantLet) {
				t.Fatalf("local reference %s %+v", name, d)
			}
			if d.ID == lambda.Params[0].ID && (name != wantParam || d.Name != wantParam) {
				t.Fatalf("parameter reference %s", name)
			}
		}
		for i := range e.Args {
			check(&e.Args[i])
		}
		if e.Opdef != nil {
			check(e.Opdef.Expr)
		}
		check(e.Body)
	}
	check(lambda)
	if m.Declarations[2].Expr.Opdef.Name != "local" {
		t.Fatal("unshadowed local renamed")
	}
	after, _ := json.Marshal(ms)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}

func TestUnshadowRecursiveCallsAndHoles(t *testing.T) {
	ms, err := ParseModules("module M { def f(a)=a val y=def f(a)=f(a); f(1) val h=(_,x)=>x }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	m := UnshadowNames(ms[0], r.Table)
	let := m.Declarations[1].Expr
	want := "f_" + let.ID.String()
	if let.Opdef.Name != want || let.Body.Opcode != want || let.Opdef.Expr.Body.Opcode != want {
		t.Fatal("recursive calls not renamed")
	}
	hole := m.Declarations[2].Expr
	if hole.Params[0].Name != "_"+"_"+hole.ID.String() {
		t.Fatal("hole not renamed like upstream")
	}
	if r.Table.Get(let.Opdef.ID).Name != "f" {
		t.Fatal("definition table entry should retain original name")
	}
}

func TestUnshadowOverrideAndAssumption(t *testing.T) {
	ms, err := ParseModules("module A { const F: int => int } module B { import A(F=(x)=>x) as I assume ok = List(1).forall(x=>x>0) }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	before, _ := json.Marshal(ms)
	m := UnshadowNames(ms[1], r.Table)
	lam := m.Declarations[0].Overrides[0].Expr
	if lam.Params[0].Name != "x_"+lam.ID.String() || lam.Body.Name != lam.Params[0].Name {
		t.Fatal("override lambda")
	}
	assumption := m.Declarations[1].Assumption
	lam = assumption.Args[1]
	if lam.Params[0].Name != "x_"+lam.ID.String() {
		t.Fatal("assumption lambda")
	}
	after, _ := json.Marshal(ms)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}

func TestUpstreamUnshadow(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/unshadow.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name          string
		Input, Output []Module
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			r, err := ResolveLinkedModules(f.Input)
			if err != nil || len(r.Errors) != 0 {
				t.Fatalf("%v %+v", err, r.Errors)
			}
			got := make([]Module, len(f.Input))
			for i, m := range f.Input {
				got[i] = UnshadowNames(m, r.Table)
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(f.Output)
			if string(a) != string(b) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
}
