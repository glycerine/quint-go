package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamPayloadState(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Module Module
		Steps  []struct {
			Operation string
			Output    *Expr
			Error     *Diagnostic
			Record    Expr
			Trace     []Expr
			RNG       string
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_payload_state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(fixture.Module, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	equal := func(label string, got RuntimeValue, expression Expr) {
		t.Helper()
		want, err := NewRuntimeValue(expression)
		if err != nil || !got.Equal(want) {
			t.Fatal(label, got.String(), want.String(), err)
		}
	}
	for index, step := range fixture.Steps {
		switch step.Operation {
		case "shift":
			if err = evaluator.Shift(); err != nil {
				t.Fatal(err)
			}
		case "restoreFirst":
			if err = evaluator.Storage.FromRecord(evaluator.Trace.States()[0]); err != nil {
				t.Fatal(err)
			}
		default:
			value, err := evaluator.Evaluate(context.Background(), step.Operation, nil)
			if step.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != step.Error.Code || diagnostic.Message != step.Error.Message || diagnostic.Reference == nil {
					t.Fatal(index, err, step.Error)
				}
			} else {
				if err != nil {
					t.Fatal(index, err)
				}
				equal("output", value, *step.Output)
			}
		}
		record, err := evaluator.Storage.Record()
		if err != nil {
			t.Fatal(err)
		}
		equal("record", record, step.Record)
		states := evaluator.Trace.States()
		if len(states) != len(step.Trace) || evaluator.RNG.State().String() != step.RNG {
			t.Fatal(index, "trace/RNG")
		}
		for i, state := range states {
			equal("trace", state, step.Trace[i])
		}
	}
}

func TestPayloadNormalFormsAndStorageIdentity(t *testing.T) {
	scalar := func(e Expr) RuntimeValue {
		t.Helper()
		v, err := NewRuntimeValue(e)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	composite := func(op string, args ...RuntimeValue) RuntimeValue {
		t.Helper()
		v, err := newRuntimeComposite(op, args)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	text := func(s string) RuntimeValue { return scalar(Expr{Kind: "str", Str: s}) }
	zero := scalar(Expr{Kind: "int"})
	pair := composite("Tup", zero, text("retained"))
	child := composite("Map", pair)
	variant := composite("variant", text("M"), child)
	normalized, err := runtimeNormalForm(variant)
	if err != nil || !sameRuntimeObject(variant, normalized) {
		t.Fatal("variant normalization changed its payload", err)
	}
	for _, op := range []string{"List", "Tup", "Rec"} {
		children := []RuntimeValue{child}
		if op == "Rec" {
			children = append([]RuntimeValue{text("x")}, children...)
		}
		value := composite(op, children...)
		normalized, err := runtimeNormalForm(value)
		if err != nil || sameRuntimeObject(value, normalized) || !value.Equal(normalized) {
			t.Fatal(op, "normal form", err)
		}
		retained, err := runtimeChildren(normalized)
		if err != nil || sameRuntimeObject(retained[len(retained)-1], child) {
			t.Fatal(op, "nested map not normalized", err)
		}
	}
	set := composite("Set", child)
	normalized, err = runtimeNormalForm(set)
	before, _ := runtimeMap(set)
	after, _ := runtimeMap(normalized)
	if err != nil || sameRuntimeObject(set, normalized) || before.root != after.root {
		t.Fatal("set normalization rebuilt its tree", err)
	}
	storage := NewVariableStorageWithMetadata(true)
	current, next := storage.Register("x", "x")
	current.Value = child
	storage.NondetPicks.Set("pick", child)
	storage.ActionTaken = "init"
	record, err := storage.Record()
	if err != nil {
		t.Fatal(err)
	}
	children, err := runtimeChildren(record)
	if err != nil {
		t.Fatal(err)
	}
	var recorded RuntimeValue
	for i := 0; i < len(children); i += 2 {
		if children[i].expression.Str == "x" {
			recorded = children[i+1]
		}
	}
	if !sameRuntimeObject(recorded, child) {
		t.Fatal("record reconstructed register value")
	}
	if err = storage.FromRecord(record); err != nil || !sameRuntimeObject(current.Value, child) {
		t.Fatal("fromRecord reconstructed register value", err)
	}
	next.Value = child
	snapshot := storage.Snapshot()
	next.Value = zero
	storage.Recover(snapshot)
	if !sameRuntimeObject(next.Value, child) {
		t.Fatal("snapshot reconstructed successor")
	}
}

func TestPayloadValidation(t *testing.T) {
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	if _, err := runtimeChildren(value); err == nil {
		t.Fatal("scalar children accepted")
	}
	if _, err := runtimeEntries(value); err == nil {
		t.Fatal("scalar entries accepted")
	}
	if _, err := runtimeNormalForm(RuntimeValue{}); err == nil {
		t.Fatal("invalid normal form")
	}
	if _, err := runtimeWithEntries("List", runtimeReferenceMap{}); err == nil {
		t.Fatal("invalid retained tree type")
	}
	for _, test := range []struct {
		op   string
		args []RuntimeValue
	}{
		{"List", []RuntimeValue{{}}}, {"Rec", []RuntimeValue{value}}, {"Rec", []RuntimeValue{value, value}},
		{"variant", nil}, {"variant", []RuntimeValue{value, value}}, {"Map", []RuntimeValue{value}}, {"unknown", nil},
	} {
		if _, err := newRuntimeComposite(test.op, test.args); err == nil {
			t.Fatal("invalid payload accepted", test.op)
		}
	}
}
