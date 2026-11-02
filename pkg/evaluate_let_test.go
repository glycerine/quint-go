package quint

import (
	"context"
	"github.com/glycerine/insdict"
	"testing"
)

func TestPureLocalValueLexicalScopeAndCleanup(t *testing.T) {
	// y captures the outer x, even when first forced under a shadowing inner x.
	literal := func(s string) Expr { n, _ := ParseInteger(s); return Expr{Kind: "int", Int: n} }
	name := func(s string) Expr { return Expr{Kind: "name", Name: s} }
	let := func(n string, value, body Expr) Expr {
		return Expr{Kind: "let", Opdef: &OperatorDef{Name: n, Qualifier: Val, Expr: &value}, Body: &body}
	}
	e := let("x", literal("2"), let("y", name("x"), let("x", literal("99"), name("y"))))
	bindings := insdict.NewDict[string, RuntimeValue]()
	outer, _ := NewRuntimeValue(literal("7"))
	bindings.Set("x", outer)
	for i := 0; i < 2; i++ {
		got, err := EvaluatePureExpression(context.Background(), e, bindings)
		if err != nil || got.String() != "2" {
			t.Fatal(got.String(), err)
		}
	}
	got, err := EvaluatePureExpression(context.Background(), name("x"), bindings)
	if err != nil || !got.Equal(outer) {
		t.Fatal("scope leaked", got, err)
	}
	// The initializer sees the preceding lexical environment, not its own binding.
	e = let("x", name("x"), name("x"))
	got, err = EvaluatePureExpression(context.Background(), e, bindings)
	if err != nil || got.String() != "7" {
		t.Fatal(got.String(), err)
	}
	e = let("x", literal("8"), Expr{Kind: "app", Opcode: "x"})
	got, err = EvaluatePureExpression(context.Background(), e, nil)
	if err == nil {
		t.Fatal("calling a value as an operator was accepted", got.String())
	}
}
func TestPureLocalValueValidation(t *testing.T) {
	value := Expr{Kind: "bool", Bool: true}
	body := Expr{Kind: "name", Name: "x"}
	for _, e := range []Expr{
		{Kind: "let"},
		{Kind: "let", Body: &body, Opdef: &OperatorDef{Name: "x", Qualifier: Nondet, Expr: &value}},
		{Kind: "let", Body: &body, Opdef: &OperatorDef{Name: "x", Expr: &Expr{Kind: "lambda"}}},
		{Kind: "let", Body: &Expr{Kind: "app", Opcode: "x", Args: []Expr{value}}, Opdef: &OperatorDef{Name: "x", Expr: &value}},
	} {
		if _, err := EvaluatePureExpression(context.Background(), e, nil); err == nil {
			t.Fatal("invalid let accepted")
		}
	}
}
