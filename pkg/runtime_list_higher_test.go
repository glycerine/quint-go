package quint

import (
	"context"
	"errors"
	"testing"
)

func TestHigherListCancellationErrorsAndOwnership(t *testing.T) {
	e, _ := ParseExpression("List(1,2,3)")
	list, _ := NewRuntimeValue(e)
	zero, _ := NewRuntimeValue(Expr{Kind: "int"})
	before := list.String()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	predicate := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		calls++
		cancel()
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	})
	if _, err := EvalListHigherBuiltin(ctx, "select", []RuntimeValue{list, predicate}); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(err, calls)
	}
	if list.String() != before {
		t.Fatal("input mutation")
	}
	failure := Diagnostic{Code: "QNT503", Message: "callback failure"}
	for _, op := range []string{"select", "foldl", "foldr"} {
		calls = 0
		callback := newRuntimeClosure(2, func(context.Context, []RuntimeValue) (RuntimeValue, error) { calls++; return RuntimeValue{}, failure })
		args := []RuntimeValue{list, zero, callback}
		if op == "select" {
			args = []RuntimeValue{list, callback}
		}
		_, err := EvalListHigherBuiltin(context.Background(), op, args)
		var d Diagnostic
		if !errors.As(err, &d) || d.Message != failure.Message || calls != 1 {
			t.Fatal(op, err, calls)
		}
	}
	boolean, _ := NewRuntimeValue(Expr{Kind: "bool"})
	nonBool := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return zero, nil })
	for _, c := range []struct {
		op   string
		args []RuntimeValue
	}{{"unknown", nil}, {"select", nil}, {"select", []RuntimeValue{{}, predicate}}, {"select", []RuntimeValue{zero, predicate}}, {"select", []RuntimeValue{list, boolean}}, {"select", []RuntimeValue{list, nonBool}}} {
		if _, err := EvalListHigherBuiltin(context.Background(), c.op, c.args); err == nil {
			t.Fatal("invalid call accepted", c.op)
		}
	}
	empty, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "List"})
	for _, op := range []string{"foldl", "foldr"} {
		got, err := EvalListHigherBuiltin(context.Background(), op, []RuntimeValue{empty, predicate, predicate})
		if err != nil || got.closure != predicate.closure {
			t.Fatal("empty fold changed initial", op, err)
		}
	}
}
