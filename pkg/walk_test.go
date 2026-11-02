package quint

import (
	"errors"
	"reflect"
	"testing"
)

func TestWalkExprOrderPruneAndFailure(t *testing.T) {
	root := &Expr{Kind: "let", Opdef: &OperatorDef{Expr: &Expr{Kind: "app", Opcode: "f", Args: []Expr{{Kind: "int"}, {Kind: "str"}}}}, Body: &Expr{Kind: "lambda", Body: &Expr{Kind: "name", Name: "x"}}}
	var got []string
	err := WalkExpr(root, func(e *Expr) (bool, error) { got = append(got, e.Kind); return true, nil })
	if err != nil || !reflect.DeepEqual(got, []string{"let", "app", "int", "str", "lambda", "name"}) {
		t.Fatalf("order %v %v", got, err)
	}
	got = nil
	if err = WalkExpr(root, func(e *Expr) (bool, error) { got = append(got, e.Kind); return e.Kind != "app", nil }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"let", "app", "lambda", "name"}) {
		t.Fatal(got)
	}
	sentinel := errors.New("stop")
	for _, kind := range []string{"let", "app", "int", "str", "lambda", "name"} {
		if err = WalkExpr(root, func(e *Expr) (bool, error) {
			if e.Kind == kind {
				return false, sentinel
			}
			return true, nil
		}); err != sentinel {
			t.Fatalf("error at %s: %v", kind, err)
		}
	}
	id, _ := ParseID("42")
	root.Body.Body.ID = id
	if FindExpr(root, id) != root.Body.Body {
		t.Fatal("finder missed nested body")
	}
	missing, _ := ParseID("99")
	if FindExpr(root, missing) != nil {
		t.Fatal("found nonexistent ID")
	}
	if err = WalkExpr(nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, e := range []*Expr{{Kind: "bad"}, {Kind: "let"}} {
		if WalkExpr(e, func(*Expr) (bool, error) { return true, nil }) == nil {
			t.Fatal("accepted malformed node")
		}
	}
}
func TestWalkTypeOrderAndFailure(t *testing.T) {
	root := &Type{Kind: "fun", Arg: &Type{Kind: "app", Ctor: &Type{Kind: "const", Name: "Box"}, Args: []Type{{Kind: "set", Elem: &Type{Kind: "int"}}}}, Res: &Type{Kind: "rec", Fields: &Row{Kind: "row", Fields: []RowField{{Name: "f", Type: Type{Kind: "oper", Args: []Type{{Kind: "str"}}, Res: &Type{Kind: "bool"}}}}, Other: &Row{Kind: "empty"}}}}
	var got []string
	if err := WalkType(root, func(typ *Type) (bool, error) { got = append(got, typ.Kind); return true, nil }); err != nil {
		t.Fatal(err)
	}
	want := []string{"fun", "app", "const", "set", "int", "rec", "oper", "str", "bool"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order: %v", got)
	}
	sentinel := errors.New("stop")
	for _, kind := range want {
		if err := WalkType(root, func(typ *Type) (bool, error) {
			if typ.Kind == kind {
				return false, sentinel
			}
			return true, nil
		}); err != sentinel {
			t.Fatalf("error at %s: %v", kind, err)
		}
	}
	count := 0
	if err := WalkType(root, func(*Type) (bool, error) { count++; return false, nil }); err != nil || count != 1 {
		t.Fatal("prune failed")
	}
	if err := WalkType(nil, nil); err != nil {
		t.Fatal(err)
	}
	if WalkType(&Type{Kind: "bad"}, func(*Type) (bool, error) { return true, nil }) == nil {
		t.Fatal("accepted bad type")
	}
}
