package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/glycerine/insdict"
)

type runtimeVariableRegistration struct {
	Key, Name string
}

type runtimeVariableSnapshot struct {
	Registration []runtimeVariableRegistration
	Names        []string
	Picks        []nondetPickSnapshot
}

type runtimeVariableDiscoveryFixture struct {
	Expressions []struct {
		Name   string
		Module Module
		Input  Expr
		runtimeVariableSnapshot
	}
	Module Module
	Steps  []struct {
		Name, RNG string
		runtimeVariableSnapshot
	}
	Selections []struct {
		Name   string
		Module Module
		RNG    string
		Record Expr
		runtimeVariableSnapshot
	}
	Registrations []struct {
		Name   string
		Inputs []runtimeVariableRegistration
		runtimeVariableSnapshot
	}
}

func loadRuntimeVariableDiscovery(t *testing.T) runtimeVariableDiscoveryFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/runtime_variable_discovery.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture runtimeVariableDiscoveryFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertRuntimeVariables(t *testing.T, storage *VariableStorage, want runtimeVariableSnapshot) {
	t.Helper()
	got := make([]runtimeVariableRegistration, 0, storage.current.Len())
	for key, register := range storage.current.All() {
		got = append(got, runtimeVariableRegistration{Key: key, Name: register.Name})
	}
	if !reflect.DeepEqual(got, want.Registration) {
		t.Errorf("registrations = %v, want %v", got, want.Registration)
	}
	if got := storage.VarNames(); !reflect.DeepEqual(got, want.Names) {
		t.Errorf("names = %v, want %v", got, want.Names)
	}
	assertNondetPicks(t, storage, want.Picks)
}

func TestUpstreamRuntimeVariableDiscovery(t *testing.T) {
	t.Parallel()
	fixture := loadRuntimeVariableDiscovery(t)
	for _, expression := range fixture.Expressions {
		t.Run(expression.Name, func(t *testing.T) {
			ctx := context.Background()
			storage := NewVariableStorageWithMetadata(true)
			variables := insdict.NewDict[string, ID]()
			for _, declaration := range expression.Module.Declarations {
				if declaration.Kind == "var" {
					variables.Set(declaration.Name, declaration.ID)
				}
			}
			var discovery RuntimeNondetDiscovery
			if err := discovery.PrepareWithVariables(ctx, expression.Input, storage, variables); err != nil {
				t.Fatal(err)
			}
			assertRuntimeVariables(t, storage, expression.runtimeVariableSnapshot)
			// The evaluator must run this same preparation automatically, and
			// constructing it must leave unused declarations undiscovered.
			evaluator, err := NewModuleEvaluator(expression.Module, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			if evaluator.Storage.current.Len() != 0 {
				t.Fatal("constructor registered unused variables")
			}
			if err := evaluator.PrepareDefinition(ctx, "result"); err != nil {
				t.Fatal(err)
			}
			assertRuntimeVariables(t, evaluator.Storage, expression.runtimeVariableSnapshot)
			if evaluator.RNG.State() != (Integer{}) {
				t.Fatal("preparation advanced RNG")
			}
		})
	}
}

func TestUpstreamRuntimeVariableCumulativeDiscovery(t *testing.T) {
	t.Parallel()
	fixture := loadRuntimeVariableDiscovery(t)
	evaluator, err := NewModuleEvaluator(fixture.Module, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for index, step := range fixture.Steps {
		if err := evaluator.PrepareDefinition(context.Background(), step.Name); err != nil {
			t.Fatal(index, err)
		}
		assertRuntimeVariables(t, evaluator.Storage, step.runtimeVariableSnapshot)
		if evaluator.RNG.State().String() != step.RNG {
			t.Fatal(index, "RNG advanced during preparation")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := evaluator.PrepareDefinition(ctx, fixture.Steps[0].Name); !errors.Is(err, context.Canceled) {
		t.Fatal("already prepared request ignored cancellation:", err)
	}
	assertRuntimeVariables(t, evaluator.Storage, fixture.Steps[len(fixture.Steps)-1].runtimeVariableSnapshot)
}

func TestUpstreamRuntimeVariableSelectedPreparation(t *testing.T) {
	t.Parallel()
	for _, selection := range loadRuntimeVariableDiscovery(t).Selections {
		t.Run(selection.Name, func(t *testing.T) {
			evaluator, err := NewModuleEvaluator(selection.Module, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if selection.Name == "test_zero_samples" {
				out, err := evaluator.Test(ctx, "zeroTest", 0)
				if err != nil || out.Status != "passed" || out.Samples != 0 || len(out.States) != 0 {
					t.Fatal(out, err)
				}
			} else {
				options := SimulationOptions{Init: "init", Step: "step", Invariant: "invariant", Witnesses: []string{"witness"}, NumberOfTraces: 1}
				if selection.Name == "simulate_zero_traces" {
					options.MaxSamples, options.NumberOfTraces = 1, 0
				}
				var out SimulationOutcome
				if selection.Name == "simulate_expressions_zero_runs" {
					// Synthetic name expressions are separate compiler requests,
					// with IDs outside this small module's declaration range.
					initID, _ := ParseID("1000001")
					stepID, _ := ParseID("1000002")
					invID, _ := ParseID("1000003")
					out, err = evaluator.SimulateExpressions(ctx, RuntimeSimulationCall{Options: options,
						Init: Expr{ID: initID, Kind: "name", Name: "init"}, Step: Expr{ID: stepID, Kind: "name", Name: "step"}, Invariant: Expr{ID: invID, Kind: "name", Name: "invariant"}})
				} else {
					out, err = evaluator.Simulate(ctx, options)
				}
				if err != nil || out.Status != "ok" || out.Samples != 0 || len(out.BestTraces) != 0 {
					t.Fatal(out, err)
				}
			}
			assertRuntimeVariables(t, evaluator.Storage, selection.runtimeVariableSnapshot)
			if evaluator.RNG.State().String() != selection.RNG {
				t.Fatal("RNG", evaluator.RNG.State().String(), selection.RNG)
			}
			record, err := evaluator.Storage.Record()
			want, wantErr := NewRuntimeValue(selection.Record)
			if err != nil || wantErr != nil || !record.Equal(want) {
				t.Fatal("record", record, err, want, wantErr)
			}
		})
	}
}

func TestUpstreamRuntimeVariableNameTraversal(t *testing.T) {
	t.Parallel()
	for _, registration := range loadRuntimeVariableDiscovery(t).Registrations {
		t.Run(registration.Name, func(t *testing.T) {
			storage := NewVariableStorage()
			for _, input := range registration.Inputs {
				current, next := storage.Register(input.Key, input.Name)
				currentAgain, nextAgain := storage.Register(input.Key, "ignored duplicate display name")
				if current != currentAgain || next != nextAgain {
					t.Fatal("duplicate registration replaced cells")
				}
			}
			assertRuntimeVariables(t, storage, registration.runtimeVariableSnapshot)
			before := storage.Snapshot()
			storage.Reset()
			storage.Shift()
			storage.Recover(before)
			assertRuntimeVariables(t, storage, registration.runtimeVariableSnapshot)
			if got := storage.VarNames(); len(got) > 0 {
				got[0] = "mutated caller slice"
				assertRuntimeVariables(t, storage, registration.runtimeVariableSnapshot)
			}
		})
	}
	var zero *VariableStorage
	if len(zero.VarNames()) != 0 || len((&VariableStorage{}).VarNames()) != 0 {
		t.Fatal("zero storage exposed variables")
	}
}

func TestRuntimeVariableDiscoveryCanceledBeforeRegistration(t *testing.T) {
	t.Parallel()
	fixture := loadRuntimeVariableDiscovery(t)
	storage := NewVariableStorage()
	variables := insdict.NewDict[string, ID]()
	variables.Set("x", fixture.Expressions[1].Module.Declarations[0].ID)
	var discovery RuntimeNondetDiscovery
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := discovery.PrepareWithVariables(ctx, fixture.Expressions[1].Input, storage, variables); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if storage.current.Len() != 0 {
		t.Fatal("canceled preparation registered a variable")
	}
}
