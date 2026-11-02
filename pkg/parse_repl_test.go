package quint

import (
	"reflect"
	"testing"
)

func TestREPLParseSelection(t *testing.T) {
	for _, source := range []string{"", " \n\t", "// comment\n", "/* comment */", "/// documentation\n"} {
		got, err := ParseExpressionOrDeclaration(source, nil)
		if err != nil || got.Kind != "none" || got.Expr != nil || got.Declarations != nil {
			t.Fatalf("%q: %+v %v", source, got, err)
		}
	}
	for _, source := range []string{"1+2", "val x = 1; x+2", "val (a,b) = pair; a", "match x { A(v) => v | _ => 0 }"} {
		got, err := ParseExpressionOrDeclaration(source, nil)
		if err != nil {
			t.Fatal(err)
		}
		want, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != "expr" || got.Declarations != nil || !reflect.DeepEqual(got.Expr, &want) {
			t.Fatalf("expression %q: %+v", source, got)
		}
	}
	for _, source := range []string{"val x = 1", "val (a,b) = pair", "type T = A | B", "import A.*", "var x: int"} {
		got, err := ParseExpressionOrDeclaration(source, nil)
		if err != nil {
			t.Fatal(err)
		}
		want, err := ParseDeclaration(source)
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != "declaration" || got.Expr != nil || !reflect.DeepEqual(got.Declarations, want) {
			t.Fatalf("declaration %q: %+v", source, got)
		}
	}
}

func TestREPLSessionIDsAndErrors(t *testing.T) {
	ids := NewIDGenerator()
	first, err := ParseExpressionOrDeclaration("val x = 1", ids)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseExpressionOrDeclaration("x", ids)
	if err != nil {
		t.Fatal(err)
	}
	if second.Expr.ID == first.Declarations[0].ID || second.Expr.ID == first.Declarations[0].Expr.ID {
		t.Fatal("session IDs reused")
	}
	for _, source := range []string{"1 2", "val x =", "module M {}", "val x = 1 val y = 2", "/// doc\nval x = 1", "@"} {
		got, err := ParseExpressionOrDeclaration(source, ids)
		if err == nil || got.Kind != "" || got.Expr != nil || got.Declarations != nil {
			t.Errorf("%q: %+v %v", source, got, err)
		}
	}
}
