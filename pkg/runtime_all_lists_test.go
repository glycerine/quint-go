package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type allListsSnapshot struct {
	Value    Expr
	Children []allListsSnapshot
	Entries  []struct {
		Key, Value allListsSnapshot
	}
}

func TestUpstreamAllListsUpTo(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name           string
		InitialCounter uint32
		Steps          []struct {
			Source       string
			Input        Expr
			Output       []allListsSnapshot
			FinalCounter uint32
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_all_lists.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	var compare func(*testing.T, RuntimeValue, allListsSnapshot)
	compare = func(t *testing.T, got RuntimeValue, want allListsSnapshot) {
		t.Helper()
		expression, err := NormalizeValue(want.Value)
		if err != nil || got.identity != normalizedValueKey(expression) {
			t.Fatal("value", got.String(), ExpressionString(expression), err)
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
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			state := &RuntimeObjectHashState{}
			if err := state.Restore(fixture.InitialCounter); err != nil {
				t.Fatal(err)
			}
			for _, step := range fixture.Steps {
				args := make([]RuntimeValue, len(step.Input.Args))
				for i, expression := range step.Input.Args {
					args[i], err = EvaluatePureExpression(context.Background(), expression, nil)
					if err != nil {
						t.Fatal(step.Source, err)
					}
				}
				result, err := EvalAllListsBuiltin(context.Background(), "allListsUpTo", args, state)
				if err != nil {
					t.Fatal(step.Source, err)
				}
				entries, err := runtimeEntries(result)
				if err != nil || len(entries) != len(step.Output) || state.Counter() != step.FinalCounter {
					t.Fatal(step.Source, "entries/counter", len(entries), state.Counter(), "want", len(step.Output), step.FinalCounter, err)
				}
				for i, output := range step.Output {
					compare(t, entries[i].Value, output)
				}
			}
		})
	}
}

func allListsValue(t *testing.T, source string) RuntimeValue {
	t.Helper()
	expression, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestAllListsValidationAndCancellation(t *testing.T) {
	ctx := context.Background()
	state := &RuntimeObjectHashState{}
	set, length, scalar := allListsValue(t, "Set(1,2)"), allListsValue(t, "3"), allListsValue(t, "true")
	for _, call := range []struct {
		op    string
		args  []RuntimeValue
		state *RuntimeObjectHashState
	}{
		{"unknown", nil, state}, {"allListsUpTo", nil, state},
		{"allListsUpTo", []RuntimeValue{{}, length}, state},
		{"allListsUpTo", []RuntimeValue{scalar, length}, state},
		{"allListsUpTo", []RuntimeValue{set, scalar}, state},
		{"allListsUpTo", []RuntimeValue{set, length}, nil},
	} {
		if _, err := EvalAllListsBuiltin(ctx, call.op, call.args, call.state); err == nil {
			t.Fatal("invalid invocation accepted", call.op)
		}
	}
	if runtimeAllListsBuiltinArity("allListsUpTo") != 2 || state.Counter() != 0 {
		t.Fatal("arity or validation side effects")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := EvalAllListsBuiltin(canceled, "allListsUpTo", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-cancellation", err)
	}
	for _, remaining := range []int{1, 2, 3, 6, 13, 46} {
		during := &allListsCancelContext{Context: ctx, remaining: remaining}
		if _, err := EvalAllListsBuiltin(during, "allListsUpTo", []RuntimeValue{set, length}, state); !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation", remaining, err)
		}
	}
}

func TestAllListsNormalizationAndOwnership(t *testing.T) {
	input := allListsValue(t, "Set(Map((2,20),(1,10)))")
	entries, _ := runtimeEntries(input)
	element := entries[0].Value
	result, err := EvalAllListsBuiltin(context.Background(), "allListsUpTo", []RuntimeValue{input, allListsValue(t, "1")}, &RuntimeObjectHashState{})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ = runtimeEntries(result)
	for _, entry := range entries {
		children, _ := runtimeChildren(entry.Value)
		if len(children) == 1 {
			if !children[0].Equal(element) || sameRuntimeObject(children[0], element) {
				t.Fatal("final set failed to normalize its nested map")
			}
			children[0] = RuntimeValue{}
			again, _ := runtimeChildren(entry.Value)
			if !again[0].Valid() {
				t.Fatal("child slice aliases list storage")
			}
		}
	}
	original, _ := runtimeEntries(input)
	if !sameRuntimeObject(original[0].Value, element) {
		t.Fatal("materialization replaced its input element")
	}
}

type allListsCancelContext struct {
	context.Context
	remaining int
}

func (c *allListsCancelContext) Err() error {
	c.remaining--
	if c.remaining < 0 {
		return context.Canceled
	}
	return nil
}
