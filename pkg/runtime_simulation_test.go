package quint

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamSimulation(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name     string
		Module   Module
		Options  SimulationOptions
		Seed     string
		Metadata bool
		Outcome  struct {
			Status     string
			Errors     []Diagnostic
			BestTraces []struct {
				States []Expr
				Result bool
				Seed   Integer
			}
			WitnessingTraces []int
			Samples          int
			TraceStatistics  struct {
				AverageTraceLength *float64
				MinTraceLength     *float64
				MaxTraceLength     *float64
			}
			ViolatedInvariants []int
		}
		RNG    string
		Record Expr
		Trace  []Expr
	}
	data, err := os.ReadFile("testdata/parser/runtime_simulation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			seed, err := ParseInteger(f.Seed)
			if err != nil {
				t.Fatal(err)
			}
			e, err := NewModuleEvaluator(f.Module, NewRNG(seed), f.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			got, err := e.Simulate(context.Background(), f.Options)
			if err != nil {
				t.Fatal(err)
			}
			want := f.Outcome
			if got.Status != want.Status || got.Samples != want.Samples || !reflect.DeepEqual(got.WitnessingTraces, want.WitnessingTraces) || !reflect.DeepEqual(got.ViolatedInvariants, want.ViolatedInvariants) {
				t.Fatalf("outcome got %+v want %+v", got, want)
			}
			if len(got.Errors) != len(want.Errors) {
				t.Fatal("errors", got.Errors, want.Errors)
			}
			for i, d := range want.Errors {
				if got.Errors[i].Code != d.Code || got.Errors[i].Message != d.Message {
					t.Fatal("error", i, got.Errors[i], d)
				}
			}
			equalStates := func(label string, values []RuntimeValue, expressions []Expr) {
				t.Helper()
				if len(values) != len(expressions) {
					t.Fatal(label, "length", len(values), len(expressions))
				}
				for i, expression := range expressions {
					want, err := NewRuntimeValue(expression)
					if err != nil || !values[i].Equal(want) {
						t.Fatal(label, i, values[i].String(), want.String(), err)
					}
				}
			}
			if len(got.BestTraces) != len(want.BestTraces) {
				t.Fatal("best trace count", len(got.BestTraces), len(want.BestTraces))
			}
			for i, trace := range want.BestTraces {
				if got.BestTraces[i].Result != trace.Result || got.BestTraces[i].Seed.String() != trace.Seed.String() {
					t.Fatal("trace", i, got.BestTraces[i], trace)
				}
				equalStates("best trace", got.BestTraces[i].States, trace.States)
			}
			equalNumber := func(label string, got float64, want *float64, empty float64) {
				t.Helper()
				if want == nil {
					if math.IsNaN(empty) {
						if !math.IsNaN(got) {
							t.Fatal(label, got, "expected NaN")
						}
					} else if got != empty {
						t.Fatal(label, got, empty)
					}
				} else if got != *want {
					t.Fatal(label, got, *want)
				}
			}
			equalNumber("average", got.TraceStatistics.AverageTraceLength, want.TraceStatistics.AverageTraceLength, math.NaN())
			equalNumber("min", got.TraceStatistics.MinTraceLength, want.TraceStatistics.MinTraceLength, math.Inf(1))
			equalNumber("max", got.TraceStatistics.MaxTraceLength, want.TraceStatistics.MaxTraceLength, 0)
			equalStates("last trace", e.Trace.States(), f.Trace)
			record, err := e.Storage.Record()
			wantRecord, err2 := NewRuntimeValue(f.Record)
			if err != nil || err2 != nil || !record.Equal(wantRecord) || e.RNG.State().String() != f.RNG {
				t.Fatal("final state", record.String(), wantRecord.String(), e.RNG.State().String(), f.RNG, err, err2)
			}
		})
	}
}

func TestSimulationValidationResetAndOwnership(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action init=x'=0 action step=x'=x+1 val inv=true }")
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := ParseInteger("37")
	e, err := NewModuleEvaluator(modules[0], NewRNG(seed), true)
	if err != nil {
		t.Fatal(err)
	}
	valid := SimulationOptions{Init: "init", Step: "step", Invariant: "inv", MaxSamples: 2, MaxSteps: 2, NumberOfTraces: 2}
	for _, alter := range []func(*SimulationOptions){
		func(o *SimulationOptions) { o.MaxSamples = -1 }, func(o *SimulationOptions) { o.MaxSteps = -1 }, func(o *SimulationOptions) { o.NumberOfTraces = -1 },
		func(o *SimulationOptions) { o.Init = "missing" }, func(o *SimulationOptions) { o.Step = "missing" }, func(o *SimulationOptions) { o.Invariant = "missing" }, func(o *SimulationOptions) { o.Witnesses = []string{"missing"} },
	} {
		options := valid
		alter(&options)
		if _, err := e.Simulate(context.Background(), options); err == nil {
			t.Fatal("invalid options accepted", options)
		}
	}
	result, err := e.Simulate(context.Background(), valid)
	if err != nil {
		t.Fatal(err)
	}
	states := e.Trace.States()
	if len(states) != 3 || len(result.BestTraces) != 2 {
		t.Fatal("trace counts", len(states), len(result.BestTraces))
	}
	result.BestTraces[0].States[0] = RuntimeValue{}
	if !e.Trace.States()[0].Equal(states[0]) {
		t.Fatal("result aliases active trace")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = e.Simulate(ctx, valid); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(e.Trace.States()) != 3 {
		t.Fatal("pre-canceled simulation mutated trace")
	}
	e.Reset()
	if len(e.Trace.States()) != 0 || e.RNG.State().String() != "37" || e.Storage.ActionTaken != "" {
		t.Fatal("reset rewound RNG or retained execution")
	}
	for _, reg := range e.Storage.current.All() {
		if reg.Value.Valid() {
			t.Fatal("reset retained variable")
		}
	}
}

type simulationCancelContext struct {
	context.Context
	remaining int
	cancel    context.CancelFunc
}

func (c *simulationCancelContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestSimulationCancellationBetweenExpressions(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action init=x'=0 action step=x'=x+1 val inv=true }")
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	limited := &simulationCancelContext{Context: ctx, remaining: 150, cancel: cancel}
	result, err := e.Simulate(limited, SimulationOptions{Init: "init", Step: "step", Invariant: "inv", MaxSamples: 1000, MaxSteps: 1000, NumberOfTraces: 1})
	if !errors.Is(err, context.Canceled) || result.Status != "error" || result.Samples >= 1000 {
		t.Fatal(result, err)
	}
}
