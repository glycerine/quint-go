package quint

import "testing"

func TestOperatorDefinitions(t *testing.T) {
	for _, q := range []string{"pure val", "pure def", "val", "def", "action", "run", "temporal", "nondet"} {
		d, err := ParseOperatorDefinition(q + " f = true")
		if err != nil || !d.Qualifier.Valid() || d.Name != "f" || d.Expr.Kind != "bool" {
			t.Fatalf("%s: %+v %v", q, d, err)
		}
	}
	d, err := ParseOperatorDefinition("pure def add(x: int, y: int): int = x + y")
	if err != nil {
		t.Fatal(err)
	}
	if d.Expr.Kind != "lambda" || d.Expr.Qualifier != PureDef || len(d.Expr.Params) != 2 || d.Expr.Params[0].TypeAnnotation.Kind != "int" || TypeString(*d.TypeAnnotation) != "(int, int) => int" {
		t.Fatalf("annotated definition %+v", d)
	}
	d, err = ParseOperatorDefinition("def f(x): int => int = x")
	if err != nil || d.TypeAnnotation.Kind != "oper" {
		t.Fatalf("old annotation %+v %v", d, err)
	}
	d, err = ParseOperatorDefinition("pure val header: bool")
	if err != nil || !d.Expr.Bool {
		t.Fatalf("header %+v %v", d, err)
	}
	for _, s := range []string{"def", "val x =", "val x = true junk"} {
		if _, err := ParseOperatorDefinition(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func TestLocalBindings(t *testing.T) {
	e, err := ParseExpression("val x = 1 val y = 2 x + y")
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != "let" || e.Opdef.Name != "x" || e.Body.Kind != "let" || e.Body.Opdef.Name != "y" || e.Body.Body.Opcode != "iadd" {
		t.Fatalf("nested bindings %+v", e)
	}
	e, err = ParseExpression("def f(x) = x + 1 f(2)")
	if err != nil || e.Opdef.Expr.Kind != "lambda" || e.Body.Opcode != "f" {
		t.Fatalf("local function %+v %v", e, err)
	}
}
