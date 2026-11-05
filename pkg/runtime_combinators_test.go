package quint

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strconv"
	"testing"
)

func TestUpstreamRuntimeSimulationCombinators(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Simulation []struct {
			Name     string
			Module   Module
			Input    Expr
			Seed     string
			Metadata bool
			Output   *Expr
			Error    *Diagnostic
			Thrown   string
			RNG      string
			Record   Expr
			Trace    []Expr
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Simulation {
		t.Run(fixture.Name, func(t *testing.T) {
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := NewModuleEvaluator(fixture.Module, NewRNG(seed), fixture.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			got, err := evaluator.EvaluateExpression(context.Background(), fixture.Input)
			if fixture.Thrown != "" {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Message != fixture.Thrown {
					t.Fatal("nonliteral limit evaluated", err)
				}
			} else if fixture.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message || diagnostic.Reference == nil || *diagnostic.Reference != *fixture.Error.Reference {
					t.Fatal("diagnostic", err, fixture.Error)
				}
			} else {
				want, failure := NewRuntimeValue(*fixture.Output)
				if err != nil || failure != nil || !got.Equal(want) {
					t.Fatal("result", got.String(), want.String(), err, failure)
				}
			}
			record, err := evaluator.Storage.Record()
			want, failure := NewRuntimeValue(fixture.Record)
			if err != nil || failure != nil || !record.Equal(want) || evaluator.RNG.State().String() != fixture.RNG {
				t.Fatal("record/RNG", record.String(), want.String(), evaluator.RNG.State(), fixture.RNG, err, failure)
			}
			trace := mustStates(&evaluator.Trace)
			if len(trace) != len(fixture.Trace) {
				t.Fatal("trace length", len(trace), len(fixture.Trace))
			}
			for i, state := range trace {
				want, failure := NewRuntimeValue(fixture.Trace[i])
				if failure != nil || !state.Equal(want) {
					t.Fatal("trace", i, state.String(), want.String(), failure)
				}
			}
		})
	}
}

func TestRuntimeCombinatorLazyRequestAndOutcome(t *testing.T) {
	t.Parallel()
	integer := func(value string) Expr { n, _ := ParseInteger(value); return Expr{Kind: "int", Int: n} }
	init := Expr{Kind: "app", Opcode: "assign", Args: []Expr{{Kind: "name", Name: "x"}, integer("0")}}
	expression := Expr{Kind: "app", Opcode: "q::testOnce", Args: []Expr{integer("2"), integer("3"), init, {Kind: "name", Name: "step"}, {Kind: "bool", Bool: true}}}
	before, _ := json.Marshal(expression)
	for _, status := range []string{"ok", "violation"} {
		got, err := EvalRuntimeCombinator(context.Background(), expression, func(_ context.Context, call RuntimeSimulationCall) (SimulationOutcome, error) {
			if call.Options.MaxSamples != 1 || call.Options.MaxSteps != 2 || call.Options.NumberOfTraces != 3 || len(call.Options.Witnesses) != 0 || call.Init.Opcode != "assign" || call.Step.Name != "step" || !call.Invariant.Bool {
				t.Fatal("lazy request", call)
			}
			call.Init.Args[0].Name = "changed"
			return SimulationOutcome{Status: status}, nil
		})
		if err != nil || got.expression.Kind != "str" || got.expression.Str != status {
			t.Fatal(got.String(), err)
		}
	}
	after, _ := json.Marshal(expression)
	if string(before) != string(after) {
		t.Fatal("simulation callback mutated expression")
	}
	first := Diagnostic{Code: "QNT503", Message: "first"}
	_, err := EvalRuntimeCombinator(context.Background(), expression, func(context.Context, RuntimeSimulationCall) (SimulationOutcome, error) {
		return SimulationOutcome{Status: "error", Errors: []Diagnostic{first, {Code: "QNT503", Message: "second"}}}, nil
	})
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Message != first.Message {
		t.Fatal("first simulation error lost", err)
	}
}

func TestRuntimeCombinatorValidationAndLimits(t *testing.T) {
	t.Parallel()
	integer := func(value string) Expr { n, _ := ParseInteger(value); return Expr{Kind: "int", Int: n} }
	expression := Expr{Kind: "app", Opcode: "q::test", Args: []Expr{integer("2"), integer("3"), integer("1"), {Kind: "bool", Bool: true}, {Kind: "bool", Bool: true}, {Kind: "bool", Bool: true}}}
	if RuntimeCombinatorArity("q::test") != 6 || RuntimeCombinatorArity("q::testOnce") != 5 || RuntimeCombinatorArity("other") != 0 {
		t.Fatal("arity inventory")
	}
	called := false
	hook := func(context.Context, RuntimeSimulationCall) (SimulationOutcome, error) {
		called = true
		return SimulationOutcome{Status: "ok"}, nil
	}
	for _, invalid := range []Expr{{Kind: "name", Name: "q::test"}, {Kind: "app", Opcode: "other"}, {Kind: "app", Opcode: "q::testOnce"}} {
		if _, err := EvalRuntimeCombinator(context.Background(), invalid, hook); err == nil || called {
			t.Fatal("malformed combinator called hook", err)
		}
	}
	for _, index := range []int{0, 1, 2} {
		invalid := copyRuntimeExpression(expression)
		invalid.Args[index] = Expr{Kind: "name", Name: "count"}
		if _, err := EvalRuntimeCombinator(context.Background(), invalid, hook); err == nil || called {
			t.Fatal("evaluated nonliteral limit", index, err)
		}
	}
	tooLarge := copyRuntimeExpression(expression)
	tooLarge.Args[0] = Expr{Kind: "int", Int: IntegerFromBig(new(big.Int).Lsh(big.NewInt(1), 1024))}
	if _, err := EvalRuntimeCombinator(context.Background(), tooLarge, hook); err == nil || called {
		t.Fatal("oversized count wrapped", err)
	}
	if strconv.IntSize == 64 {
		rounded := copyRuntimeExpression(expression)
		rounded.Args[0] = integer("9007199254740993")
		if _, err := EvalRuntimeCombinator(context.Background(), rounded, func(_ context.Context, call RuntimeSimulationCall) (SimulationOutcome, error) {
			if int64(call.Options.MaxSamples) != int64(9007199254740992) {
				t.Fatal("Number rounding", call.Options.MaxSamples)
			}
			return SimulationOutcome{Status: "ok"}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvalRuntimeCombinator(canceled, expression, hook); !errors.Is(err, context.Canceled) || called {
		t.Fatal("canceled combinator called hook", err)
	}
	if _, err := EvalRuntimeCombinator(context.Background(), expression, nil); err == nil {
		t.Fatal("nil simulation hook accepted")
	}
	for _, hook := range []RuntimeSimulationHook{
		func(context.Context, RuntimeSimulationCall) (SimulationOutcome, error) {
			return SimulationOutcome{}, context.Canceled
		},
		func(context.Context, RuntimeSimulationCall) (SimulationOutcome, error) {
			return SimulationOutcome{Status: "error"}, nil
		},
	} {
		if _, err := EvalRuntimeCombinator(context.Background(), expression, hook); err == nil {
			t.Fatal("simulation error hidden")
		}
	}
}

func TestRuntimeExpressionRequestsAndSimulation(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules("module M { var x:int action init=x'=0 action step=x'=x+1 val inv=true val even=x%2==0 val read=x }")
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	name := func(value string) Expr { return Expr{Kind: "name", Name: value} }
	call := RuntimeSimulationCall{
		Options: SimulationOptions{MaxSamples: 2, MaxSteps: 2, NumberOfTraces: 1, Witnesses: []string{"even"}},
		Init:    name("init"), Step: name("step"), Invariant: name("inv"),
	}
	outcome, err := evaluator.SimulateExpressions(ctx, call)
	if err != nil || outcome.Status != "ok" || outcome.Samples != 2 || len(outcome.BestTraces) != 1 || len(outcome.BestTraces[0].States) != 3 || len(outcome.WitnessingTraces) != 1 || outcome.WitnessingTraces[0] != 2 {
		t.Fatal("expression simulation", outcome, err)
	}
	value, err := evaluator.EvaluateExpression(ctx, name("read"))
	if err != nil || value.String() != "2" {
		t.Fatal("ordinary expression request lost state", value.String(), err)
	}
	nested, err := ParseExpression("List(q::testOnce(0,1,true,true,true))")
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluator.EvaluateExpression(ctx, nested)
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT000" || diagnostic.Message != "Unknown builtin q::testOnce" {
		t.Fatal("nested combinator was intercepted", err)
	}
	for _, options := range []SimulationOptions{
		{MaxSamples: -1}, {MaxSteps: -1}, {NumberOfTraces: -1}, {Witnesses: []string{"missing"}},
	} {
		invalid := call
		invalid.Options = options
		if _, err := evaluator.SimulateExpressions(ctx, invalid); err == nil {
			t.Fatal("invalid expression simulation accepted", options)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := evaluator.SimulateExpressions(canceled, call); !errors.Is(err, context.Canceled) {
		t.Fatal("expression simulation ignored cancellation", err)
	}
}
