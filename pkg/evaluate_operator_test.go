package quint

import (
	"context"
	"testing"
)

func TestLocalOperatorCaptureAndParameters(t *testing.T) {
	integer := func(text string) Expr { n, _ := ParseInteger(text); return Expr{Kind: "int", Int: n} }
	name := func(text string) Expr { return Expr{Kind: "name", Name: text} }
	let := func(n string, value, body Expr) Expr {
		return Expr{Kind: "let", Opdef: &OperatorDef{Name: n, Qualifier: Def, Expr: &value}, Body: &body}
	}
	sum := Expr{Kind: "app", Opcode: "iadd", Args: []Expr{name("x"), name("y")}}
	lambda := Expr{Kind: "lambda", Params: []Parameter{{Name: "y"}}, Body: &sum}
	call := Expr{Kind: "app", Opcode: "f", Args: []Expr{integer("3")}}
	expression := let("x", integer("2"), let("f", lambda, let("x", integer("99"), call)))
	for i := 0; i < 2; i++ {
		got, err := EvaluatePureExpression(context.Background(), expression, nil)
		if err != nil || got.String() != "5" {
			t.Fatal(got.String(), err)
		}
	}
	// An argument evaluates in the calling scope before the callee installs params.
	call.Args = []Expr{name("x")}
	expression = let("x", integer("2"), let("f", lambda, let("x", integer("99"), call)))
	got, err := EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil || got.String() != "101" {
		t.Fatal(got.String(), err)
	}
	// Parameter bindings shadow captured names, with no cache carried across calls.
	lambda.Params = []Parameter{{Name: "x"}, {Name: "y"}}
	first := Expr{Kind: "app", Opcode: "f", Args: []Expr{integer("3"), integer("4")}}
	second := Expr{Kind: "app", Opcode: "f", Args: []Expr{integer("5"), integer("6")}}
	body := Expr{Kind: "app", Opcode: "List", Args: []Expr{first, second}}
	expression = let("x", integer("99"), let("f", lambda, body))
	got, err = EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil || got.String() != "List(7, 11)" {
		t.Fatal(got.String(), err)
	}
	first.Args = nil
	expression = let("f", lambda, first)
	if _, err = EvaluatePureExpression(context.Background(), expression, nil); err == nil {
		t.Fatal("arity accepted")
	}
}
