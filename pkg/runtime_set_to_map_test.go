package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamSetToMap(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name           string
		InitialCounter uint32
		Steps          []struct {
			Source       string
			Input        Expr
			Output       *collectionPayloadSnapshot
			Error        *Diagnostic
			FinalCounter uint32
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_set_to_map.json")
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
		t.Run(fixture.Name, func(t *testing.T) {
			state := &RuntimeObjectHashState{}
			if err := state.Restore(fixture.InitialCounter); err != nil {
				t.Fatal(err)
			}
			for _, step := range fixture.Steps {
				input, err := EvaluatePureExpression(context.Background(), step.Input.Args[0], nil)
				if err != nil {
					t.Fatal(step.Source, err)
				}
				result, err := EvalSetToMapBuiltin(context.Background(), []RuntimeValue{input}, state)
				if state.Counter() != step.FinalCounter {
					t.Fatal(step.Source, "counter", state.Counter(), "want", step.FinalCounter)
				}
				if step.Error != nil {
					var diagnostic Diagnostic
					if !errors.As(err, &diagnostic) || diagnostic.Code != step.Error.Code || diagnostic.Message != step.Error.Message {
						t.Fatal(step.Source, err, step.Error)
					}
					continue
				}
				if err != nil {
					t.Fatal(step.Source, err)
				}
				compare(t, result, *step.Output)
			}
		})
	}
}

func TestSetToMapValidationAndCancellation(t *testing.T) {
	ctx := context.Background()
	state := &RuntimeObjectHashState{}
	input := allListsValue(t, "Set((1,10),(2,20))")
	scalar := allListsValue(t, "7")
	for _, call := range []struct {
		args  []RuntimeValue
		state *RuntimeObjectHashState
	}{
		{nil, state}, {[]RuntimeValue{input, input}, state}, {[]RuntimeValue{{}}, state},
		{[]RuntimeValue{scalar}, state}, {[]RuntimeValue{input}, nil},
	} {
		if _, err := EvalSetToMapBuiltin(ctx, call.args, call.state); err == nil {
			t.Fatal("invalid invocation accepted")
		}
	}
	if state.Counter() != 0 {
		t.Fatal("validation allocated object hashes")
	}
	for _, remaining := range []int{0, 1, 3, 5} {
		during := &allListsCancelContext{Context: ctx, remaining: remaining}
		if _, err := EvalSetToMapBuiltin(during, []RuntimeValue{input}, state); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation", remaining, err)
		}
	}
}

func TestSetToMapNormalizesKeysAndPreservesValues(t *testing.T) {
	ctx := context.Background()
	key := allListsValue(t, "Map((2,20),(1,10))")
	callback := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		return args[0], nil
	})
	pair, err := newRuntimeComposite("Tup", []RuntimeValue{key, callback})
	if err != nil {
		t.Fatal(err)
	}
	input, err := newRuntimeComposite("Set", []RuntimeValue{pair})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := runtimeMap(input)
	inputEntries, _ := runtimeEntries(input)
	inputPair, _ := runtimeChildren(inputEntries[0].Value)
	result, err := EvalSetToMapBuiltin(ctx, []RuntimeValue{input}, &RuntimeObjectHashState{})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := runtimeEntries(result)
	if len(entries) != 1 || !entries[0].Key.Equal(inputPair[0]) || sameRuntimeObject(entries[0].Key, inputPair[0]) || !sameRuntimeObject(entries[0].Value, callback) {
		t.Fatal("conversion failed to normalize its key or preserve its value")
	}
	value, err := EvalMapBuiltin("get", []RuntimeValue{result, key})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := value.Apply(ctx, []RuntimeValue{key})
	if err != nil || !sameRuntimeObject(applied, key) {
		t.Fatal("stored callback no longer executes", err)
	}
	view := result.Expression()
	view.Args[0].Args[0] = Expr{Kind: "bool", Bool: true}
	after, _ := runtimeMap(input)
	if before.root != after.root || !sameRuntimeObject(entries[0].Value, callback) {
		t.Fatal("conversion mutated its input or exposed storage")
	}
}
