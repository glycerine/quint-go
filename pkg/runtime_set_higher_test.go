package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamHigherSetEvaluation(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Input  Expr
		Output *collectionPayloadSnapshot
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/runtime_set_higher.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var compare func(*testing.T, RuntimeValue, collectionPayloadSnapshot)
	compare = func(t *testing.T, got RuntimeValue, want collectionPayloadSnapshot) {
		t.Helper()
		expression, err := NormalizeValue(want.Value)
		if err != nil || got.identity != normalizedValueKey(expression) {
			t.Fatal("value", got.String(), ExpressionString(expression), err)
		}
		if want.Entries != nil {
			entries, err := runtimeEntries(got)
			if err != nil || len(entries) != len(want.Entries) {
				t.Fatal("entries", len(entries), len(want.Entries), err)
			}
			for i, entry := range want.Entries {
				compare(t, entries[i].Key, entry.Key)
				compare(t, entries[i].Value, entry.Value)
			}
		}
		if want.Children != nil {
			children, err := runtimeChildren(got)
			if err != nil || len(children) != len(want.Children) {
				t.Fatal("children", len(children), len(want.Children), err)
			}
			for i, child := range want.Children {
				compare(t, children[i], child)
			}
		}
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Source, func(t *testing.T) {
			got, err := EvaluatePureExpression(context.Background(), fixture.Input, nil)
			if fixture.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message || diagnostic.Reference == nil {
					t.Fatal(err, fixture.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			compare(t, got, *fixture.Output)
		})
	}
}

func higherSetValue(t *testing.T, source string) RuntimeValue {
	t.Helper()
	expression, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestHigherSetQuantifierCallbackAndReductionOrder(t *testing.T) {
	input := higherSetValue(t, "Set(1,2,3)")
	zero := higherSetValue(t, "0")
	failure := Diagnostic{Code: "QNT503", Message: "callback failure"}
	isFailure := func(err error) bool {
		var diagnostic Diagnostic
		return errors.As(err, &diagnostic) && diagnostic.Code == failure.Code && diagnostic.Message == failure.Message
	}
	for _, op := range []string{"exists", "forall"} {
		calls := 0
		predicate := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
			calls++
			return NewRuntimeValue(Expr{Kind: "bool", Bool: op == "exists"})
		})
		got, err := EvalSetHigherBuiltin(context.Background(), op, []RuntimeValue{input, predicate})
		if err != nil || calls != 3 || got.expression.Bool != (op == "exists") {
			t.Fatal(op, got.String(), err, calls)
		}
		// A decisive first Boolean cannot suppress later callback failures.
		calls = 0
		predicate = newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
			calls++
			if calls == 2 {
				return RuntimeValue{}, failure
			}
			return NewRuntimeValue(Expr{Kind: "bool", Bool: op == "exists"})
		})
		if _, err = EvalSetHigherBuiltin(context.Background(), op, []RuntimeValue{input, predicate}); !isFailure(err) || calls != 2 {
			t.Fatal(op, err, calls)
		}
		// Boolean conversion happens after all callbacks, including failures.
		calls = 0
		predicate = newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
			calls++
			if calls == 2 {
				return RuntimeValue{}, failure
			}
			return zero, nil
		})
		if _, err = EvalSetHigherBuiltin(context.Background(), op, []RuntimeValue{input, predicate}); !isFailure(err) || calls != 2 {
			t.Fatal(op, err, calls)
		}
	}
}

func TestHigherSetCancellationAndValidation(t *testing.T) {
	input := higherSetValue(t, "Set(1,2,3)")
	empty := higherSetValue(t, "Set()")
	zero := higherSetValue(t, "0")
	truth := higherSetValue(t, "true")
	for _, op := range []string{"exists", "forall", "map", "filter", "fold", "mapBy"} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		callbackArity := 1
		if op == "fold" {
			callbackArity = 2
		}
		callback := newRuntimeClosure(callbackArity, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
			calls++
			cancel()
			return truth, nil
		})
		args := []RuntimeValue{input, callback}
		if op == "fold" {
			args = []RuntimeValue{input, zero, callback}
		}
		if _, err := EvalSetHigherBuiltin(ctx, op, args); !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatal(op, err, calls)
		}
		if _, err := EvalSetHigherBuiltin(ctx, op, nil); !errors.Is(err, context.Canceled) {
			t.Fatal(op, "pre-cancellation", err)
		}
		if op != "fold" {
			if _, err := EvalSetHigherBuiltin(context.Background(), op, []RuntimeValue{empty, zero}); err == nil {
				t.Fatal(op, "empty collection accepted non-lambda")
			}
		}
	}
	callback := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return zero, nil })
	got, err := EvalSetHigherBuiltin(context.Background(), "fold", []RuntimeValue{empty, callback, zero})
	if err != nil || !sameRuntimeObject(got, callback) {
		t.Fatal("empty fold changed initial value or resolved callback", err)
	}
	for _, test := range []struct {
		op   string
		args []RuntimeValue
	}{
		{"unknown", nil}, {"fold", nil}, {"exists", []RuntimeValue{{}, callback}},
		{"map", []RuntimeValue{zero, callback}}, {"fold", []RuntimeValue{input, zero, zero}},
		{"filter", []RuntimeValue{input, callback}}, {"exists", []RuntimeValue{input, callback}},
		{"forall", []RuntimeValue{input, callback}}, {"mapBy", []RuntimeValue{empty, {}}},
	} {
		if _, err := EvalSetHigherBuiltin(context.Background(), test.op, test.args); err == nil {
			t.Fatal("invalid call accepted", test.op)
		}
	}
}

func TestHigherSetPayloadOwnership(t *testing.T) {
	ctx := context.Background()
	input := higherSetValue(t, "Set(Map((2,20),(1,10)))")
	entries, err := runtimeEntries(input)
	if err != nil {
		t.Fatal(err)
	}
	element := entries[0].Value
	before, _ := runtimeMap(input)
	callback := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		if !sameRuntimeObject(args[0], element) {
			t.Fatal("callback rebuilt input element")
		}
		return element, nil
	})
	output, err := EvalSetHigherBuiltin(ctx, "mapBy", []RuntimeValue{input, callback})
	if err != nil {
		t.Fatal(err)
	}
	pairs, _ := runtimeEntries(output)
	if len(pairs) != 1 || !sameRuntimeObject(pairs[0].Value, element) || sameRuntimeObject(pairs[0].Key, element) {
		t.Fatal("mapBy must normalize its key and retain its callback value")
	}
	truth := higherSetValue(t, "true")
	predicate := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return truth, nil })
	for _, op := range []string{"map", "filter"} {
		f := callback
		if op == "filter" {
			f = predicate
		}
		got, err := EvalSetHigherBuiltin(ctx, op, []RuntimeValue{input, f})
		if err != nil {
			t.Fatal(op, err)
		}
		children, _ := runtimeEntries(got)
		if len(children) != 1 || !children[0].Value.Equal(element) || sameRuntimeObject(children[0].Value, element) {
			t.Fatal(op, "did not normalize selected map")
		}
	}
	initial := higherSetValue(t, "Map((9,90))")
	reducer := newRuntimeClosure(2, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		if !sameRuntimeObject(args[0], initial) || !sameRuntimeObject(args[1], element) {
			t.Fatal("fold rebuilt accumulator/element or reversed arguments")
		}
		return element, nil
	})
	got, err := EvalSetHigherBuiltin(ctx, "fold", []RuntimeValue{input, initial, reducer})
	if err != nil || !sameRuntimeObject(got, element) {
		t.Fatal("fold rebuilt callback result", err)
	}
	after, _ := runtimeMap(input)
	if before.root != after.root {
		t.Fatal("operation mutated input tree")
	}
}
