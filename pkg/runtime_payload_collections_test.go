package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type collectionPayloadSnapshot struct {
	Value    Expr
	Entries  []collectionPayloadEntry
	Children []collectionPayloadSnapshot
}
type collectionPayloadEntry struct {
	Key   collectionPayloadSnapshot
	Value collectionPayloadSnapshot
}

func TestUpstreamPayloadCollections(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Input  Expr
		Output *collectionPayloadSnapshot
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/runtime_payload_collections.json")
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
				t.Fatal("entry count", len(entries), len(want.Entries), err)
			}
			for i, entry := range want.Entries {
				compare(t, entries[i].Key, entry.Key)
				compare(t, entries[i].Value, entry.Value)
			}
		}
		if want.Children != nil {
			children, err := runtimeChildren(got)
			if err != nil || len(children) != len(want.Children) {
				t.Fatal("child count", len(children), len(want.Children), err)
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

func TestPayloadMapStoredValuesAndNoop(t *testing.T) {
	key, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	value, _ := NewRuntimeValue(Expr{Kind: "str", Str: "retained"})
	pair, err := newRuntimeComposite("Tup", []RuntimeValue{key, value})
	if err != nil {
		t.Fatal(err)
	}
	input, err := newRuntimeComposite("Map", []RuntimeValue{pair})
	if err != nil {
		t.Fatal(err)
	}
	got, err := EvalMapBuiltin("get", []RuntimeValue{input, key})
	if err != nil || !sameRuntimeObject(got, value) {
		t.Fatal("get rebuilt stored value", err)
	}
	before, err := runtimeMap(input)
	if err != nil {
		t.Fatal(err)
	}
	output, err := EvalMapBuiltin("put", []RuntimeValue{input, key, got})
	if err != nil {
		t.Fatal(err)
	}
	after, err := runtimeMap(output)
	if err != nil || before.root != after.root {
		t.Fatal("same-object put changed map tree", err)
	}
	fresh, _ := NewRuntimeValue(Expr{Kind: "str", Str: "retained"})
	output, err = EvalMapBuiltin("put", []RuntimeValue{input, key, fresh})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := EvalMapBuiltin("get", []RuntimeValue{output, key})
	original, err2 := EvalMapBuiltin("get", []RuntimeValue{input, key})
	if err != nil || err2 != nil || !sameRuntimeObject(updated, fresh) || !sameRuntimeObject(original, value) {
		t.Fatal("put lost payload or mutated parent", err, err2)
	}
}
