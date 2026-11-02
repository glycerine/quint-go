package quint

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeClosureLifetimeOwnershipAndCancellation(t *testing.T) {
	expression, err := ParseExpression("{ val x=2; (y)=>x+y }")
	if err != nil {
		t.Fatal(err)
	}
	closure, err := EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil || !closure.Valid() {
		t.Fatal(err)
	}
	if closure.Equal(closure) {
		t.Fatal("reference closures are not structurally equal")
	}
	number, _ := ParseInteger("3")
	argument, _ := NewRuntimeValue(Expr{Kind: "int", Int: number})
	for i := 0; i < 2; i++ {
		got, err := closure.Apply(context.Background(), []RuntimeValue{argument})
		if err != nil || got.String() != "5" {
			t.Fatal(got.String(), err)
		}
	}
	exported := closure.Expression()
	exported.Body.Str = "mutated"
	exported.Params[0].Name = "bad"
	got, err := closure.Apply(context.Background(), []RuntimeValue{argument})
	if err != nil || got.String() != "5" {
		t.Fatal(got.String(), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := closure.Apply(ctx, []RuntimeValue{argument}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := closure.Apply(context.Background(), nil); err == nil {
		t.Fatal("arity accepted")
	}
	if _, err := argument.Apply(context.Background(), nil); err == nil {
		t.Fatal("nonclosure accepted")
	}
	if _, err := closure.Apply(context.Background(), []RuntimeValue{{}}); err == nil {
		t.Fatal("invalid argument accepted")
	}
	// Creating with a canceled-later context does not poison subsequent calls.
	creation, cancelCreation := context.WithCancel(context.Background())
	fresh, err := EvaluatePureExpression(creation, expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancelCreation()
	got, err = fresh.Apply(context.Background(), []RuntimeValue{argument})
	if err != nil || got.String() != "5" {
		t.Fatal(got.String(), err)
	}
}
