package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

type objectEvaluationOutcome struct {
	Output  *collectionPayloadSnapshot
	Error   *Diagnostic
	Counter uint32
}

func compareObjectEvaluationValue(t *testing.T, got RuntimeValue, want collectionPayloadSnapshot) {
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
			compareObjectEvaluationValue(t, entries[i].Key, entry.Key)
			compareObjectEvaluationValue(t, entries[i].Value, entry.Value)
		}
	}
	if want.Children != nil {
		children, err := runtimeChildren(got)
		if err != nil || len(children) != len(want.Children) {
			t.Fatal("children", len(children), len(want.Children), err)
		}
		for i, child := range want.Children {
			compareObjectEvaluationValue(t, children[i], child)
		}
	}
}

func compareObjectEvaluation(t *testing.T, value RuntimeValue, err error, state *RuntimeObjectHashState, want objectEvaluationOutcome) {
	t.Helper()
	if state.Counter() != want.Counter {
		t.Fatal("counter", state.Counter(), "want", want.Counter)
	}
	if want.Error != nil {
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Code != want.Error.Code || diagnostic.Message != want.Error.Message || diagnostic.Reference == nil {
			t.Fatal(err, want.Error)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	compareObjectEvaluationValue(t, value, *want.Output)
}

func TestUpstreamObjectStateEvaluation(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Pure []struct {
			Name           string
			InitialCounter uint32
			Steps          []struct {
				Source string
				Input  Expr
				objectEvaluationOutcome
			}
		}
		Closures []struct {
			Name                            string
			InitialCounter, CreationCounter uint32
			Input                           Expr
			Runs                            []struct {
				Arguments []Expr
				objectEvaluationOutcome
			}
		}
		Module  Module
		Modules []struct {
			InitialCounter uint32
			Steps          []struct {
				Operation   string
				Serialized  *Expr
				Record      Expr
				TraceLength int
				objectEvaluationOutcome
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_object_evaluation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, scenario := range fixture.Pure {
		t.Run(scenario.Name, func(t *testing.T) {
			state := &RuntimeObjectHashState{}
			if err := state.Restore(scenario.InitialCounter); err != nil {
				t.Fatal(err)
			}
			for _, step := range scenario.Steps {
				value, err := EvaluatePureExpressionWithRuntimeState(ctx, step.Input, nil, state)
				compareObjectEvaluation(t, value, err, state, step.objectEvaluationOutcome)
			}
		})
	}
	for _, scenario := range fixture.Closures {
		t.Run(scenario.Name, func(t *testing.T) {
			state := &RuntimeObjectHashState{}
			if err := state.Restore(scenario.InitialCounter); err != nil {
				t.Fatal(err)
			}
			closure, err := EvaluatePureExpressionWithRuntimeState(ctx, scenario.Input, nil, state)
			if err != nil || closure.closure == nil || state.Counter() != scenario.CreationCounter {
				t.Fatal("creation", err, state.Counter(), scenario.CreationCounter)
			}
			for _, run := range scenario.Runs {
				arguments := make([]RuntimeValue, len(run.Arguments))
				for i, expression := range run.Arguments {
					arguments[i], err = NewRuntimeValue(expression)
					if err != nil {
						t.Fatal(err)
					}
				}
				value, err := closure.Apply(ctx, arguments)
				compareObjectEvaluation(t, value, err, state, run.objectEvaluationOutcome)
			}
		})
	}
	for i, scenario := range fixture.Modules {
		t.Run(fmt.Sprintf("module_%d", i), func(t *testing.T) {
			evaluator, err := NewModuleEvaluator(fixture.Module, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			state := evaluator.ObjectHashState()
			if err := state.Restore(scenario.InitialCounter); err != nil {
				t.Fatal(err)
			}
			var closure, savedValue RuntimeValue
			var snapshot VariableSnapshot
			zero, _ := NewRuntimeValue(Expr{Kind: "int"})
			for _, step := range scenario.Steps {
				switch step.Operation {
				case "shift":
					if err := evaluator.Shift(); err != nil {
						t.Fatal(err)
					}
				case "snapshot":
					snapshot = evaluator.Storage.Snapshot()
				case "recover":
					evaluator.Storage.Recover(snapshot)
				case "reset":
					evaluator.Reset()
				case "getClosure":
					closure, err = evaluator.Evaluate(ctx, "thunk", nil)
					if err != nil || closure.closure == nil {
						t.Fatal(err)
					}
				case "applyClosure":
					value, err := closure.Apply(ctx, []RuntimeValue{zero})
					compareObjectEvaluation(t, value, err, state, step.objectEvaluationOutcome)
				case "serialize":
					output, err := SerializeRuntimeValue(savedValue, state)
					actual, _ := json.Marshal(output)
					want, _ := json.Marshal(step.Serialized)
					if err != nil || !bytes.Equal(actual, want) {
						t.Fatal("serialized", string(actual), string(want), err)
					}
				default:
					var arguments []RuntimeValue
					if step.Operation == "convert" {
						arguments = []RuntimeValue{zero}
					}
					value, err := evaluator.Evaluate(ctx, step.Operation, arguments)
					compareObjectEvaluation(t, value, err, state, step.objectEvaluationOutcome)
					if err == nil {
						savedValue = value
					}
				}
				if state.Counter() != step.Counter || evaluator.ObjectHashState() != state || len(mustStates(&evaluator.Trace)) != step.TraceLength {
					t.Fatal(step.Operation, "state/trace", state.Counter(), step.Counter, len(mustStates(&evaluator.Trace)), step.TraceLength)
				}
				record, err := evaluator.Storage.Record()
				want, wantErr := NewRuntimeValue(step.Record)
				if err != nil || wantErr != nil || !record.Equal(want) {
					t.Fatal(step.Operation, "record", record.String(), want.String(), err, wantErr)
				}
			}
		})
	}
}

func TestObjectStateStorageOwnershipAndPureDefaults(t *testing.T) {
	ctx := context.Background()
	expression, err := ParseExpression("Set((1,0),(1,1),(1,2),(1,3),(1,4),(1,5),(1,6),(1,7),(1,8)).setToMap().get(1)")
	if err != nil {
		t.Fatal(err)
	}
	first, err := EvaluatePureExpression(ctx, expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := EvaluatePureExpressionWithRuntimeState(ctx, expression, nil, nil)
	if err != nil || !first.Equal(again) {
		t.Fatal("nil state did not start a fresh execution", err)
	}
	storage := NewVariableStorage()
	state := storage.ObjectHashState()
	for i := uint32(1); i <= 3; i++ {
		_, err := EvaluateStateExpression(ctx, expression, storage)
		if err != nil || storage.ObjectHashState() != state || state.Counter() != i*9 {
			t.Fatal("stateful call restarted object hashes", i, state.Counter(), err)
		}
	}
	storage.Reset()
	storage.Shift()
	snapshot := storage.Snapshot()
	storage.Recover(snapshot)
	if state.Counter() != 27 {
		t.Fatal("state lifecycle rewound object hashes", state.Counter())
	}
	if _, err := EvaluateStateExpressionWithRNG(ctx, expression, storage, NewRNG(Integer{})); err != nil || state.Counter() != 36 {
		t.Fatal("RNG entry point replaced object hashes", err, state.Counter())
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := EvaluatePureExpressionWithRuntimeState(canceled, expression, nil, state); !errors.Is(err, context.Canceled) || state.Counter() != 36 {
		t.Fatal("pre-cancellation changed shared state", err, state.Counter())
	}
}

func TestObjectStateSharedAcrossModuleExecutions(t *testing.T) {
	ctx := context.Background()
	modules, err := ParseModules("module M { pure def convert(n)=Set((1,0),(1,1),(1,2),(1,3),(1,4),(1,5),(1,6),(1,7),(1,8)).setToMap().get(1) }")
	if err != nil {
		t.Fatal(err)
	}
	state := &RuntimeObjectHashState{}
	first, err := NewModuleEvaluatorWithRuntimeState(modules[0], nil, false, state)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewModuleEvaluatorWithRuntimeState(modules[0], nil, false, state)
	if err != nil {
		t.Fatal(err)
	}
	zero, _ := NewRuntimeValue(Expr{Kind: "int"})
	for i, evaluator := range []*ModuleEvaluator{first, second, first} {
		if _, err := evaluator.Evaluate(ctx, "convert", []RuntimeValue{zero}); err != nil || evaluator.ObjectHashState() != state || state.Counter() != uint32(i+1)*9 {
			t.Fatal("shared execution counter", i, state.Counter(), err)
		}
	}
	first.Reset()
	if second.ObjectHashState().Counter() != 27 {
		t.Fatal("reset replaced the shared owner")
	}
	independent, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil || independent.ObjectHashState() == state || independent.ObjectHashState().Counter() != 0 {
		t.Fatal("default module inherited unrelated execution history", err)
	}
	if _, err := independent.Evaluate(ctx, "convert", []RuntimeValue{zero}); err != nil || independent.ObjectHashState().Counter() != 9 || state.Counter() != 27 {
		t.Fatal("independent execution changed shared history", err)
	}
}

func TestHistoricalSetToMapCorpus(t *testing.T) {
	t.Parallel()
	// The original oracle evaluated these cases in one Node process. A shared
	// state reproduces that sequence; restarting at zero for each case does not.
	var fixtures []struct {
		Source string
		Input  Expr
		Output *Expr
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/runtime_set_to_map_pending.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	state := &RuntimeObjectHashState{}
	for _, fixture := range fixtures {
		t.Run(fixture.Source, func(t *testing.T) {
			value, err := EvaluatePureExpressionWithRuntimeState(context.Background(), fixture.Input, nil, state)
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
			want, err := NewRuntimeValue(*fixture.Output)
			if err != nil || !value.Equal(want) {
				t.Fatal(value.String(), want.String(), err)
			}
		})
	}
	if len(fixtures) != 19 || state.Counter() != 151 {
		t.Fatal("historical fixture process sequence changed", len(fixtures), state.Counter())
	}
}
