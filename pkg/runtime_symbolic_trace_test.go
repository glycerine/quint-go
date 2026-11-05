package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

type symbolicTraceEvent struct {
	Index    int
	Status   string
	Vars     []string
	States   []Expr
	TestName string
	ITF      struct {
		Value json.RawMessage
		Error string
	}
}

type symbolicTraceDrive struct {
	InitialUID, FinalUID uint32
	RNG                  string
	TraceLength          int
	BestTraceLength      int
	Thrown               *symbolicNondetException
	Events               []symbolicTraceEvent
	Outcome              *struct {
		Status     string
		Errors     []Diagnostic
		Samples    int
		BestTraces []struct {
			States []Expr
			Result bool
			Seed   Integer
		}
	}
	Result *struct {
		Name, Status string
		Errors       []Diagnostic
		Seed         Integer
		NSamples     int
	}
}

type symbolicTraceCase struct {
	Name, Source, Seed, Classification string
	Metadata                           bool
	TestOnly                           bool
	MaxSamples, MaxSteps               int
	AnalysisErrors                     []Diagnostic
	Simulation                         *symbolicTraceDrive
	Test                               symbolicTraceDrive
}

func symbolicTraceJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func assertSymbolicTraceException(t *testing.T, err error, thrown *symbolicNondetException) {
	t.Helper()
	if thrown == nil {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var exception RuntimeDomainException
	if !errors.As(err, &exception) || exception.Name != thrown.Name || exception.Message != thrown.Message {
		t.Fatal("exception", err, "want", *thrown)
	}
}

func assertSymbolicTraceITF(t *testing.T, event symbolicTraceEvent, states []Expr, metadata bool) {
	t.Helper()
	if symbolicTraceJSON(t, states) != symbolicTraceJSON(t, event.States) {
		t.Fatalf("serialized trace\ngot  %s\nwant %s", symbolicTraceJSON(t, states), symbolicTraceJSON(t, event.States))
	}
	encoded, err := EncodeITFTrace(event.Vars, states, metadata)
	if event.ITF.Error != "" {
		if err == nil || err.Error() != event.ITF.Error {
			t.Fatal("ITF error", err, event.ITF.Error)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err = json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(event.ITF.Value, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("ITF\ngot  %s\nwant %s", encoded, event.ITF.Value)
	}
}

func TestUpstreamSymbolicSourceTraceSerialization(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []symbolicTraceCase
	}
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_trace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			parsed, err := ParseModulesWithLocations("fixture.qnt", fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			linked, err := ResolveLinkedModules(parsed.Modules)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			analysis, err := AnalyzeModules(parsed.Modules, linked.Table)
			if err != nil || len(analysis.Diagnostics) != len(fixture.AnalysisErrors) {
				t.Fatal("static checking", err, analysis.Diagnostics)
			}
			for i, diagnostic := range analysis.Diagnostics {
				want := fixture.AnalysisErrors[i]
				if diagnostic.Code != want.Code || !reflect.DeepEqual(diagnostic.Reference, want.Reference) {
					t.Fatal("direct API static diagnostic", diagnostic, want)
				}
			}
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"simulation", "test"} {
				if mode == "simulation" && fixture.Simulation == nil {
					continue
				}
				t.Run(mode, func(t *testing.T) {
					want := fixture.Test
					if mode == "simulation" {
						want = *fixture.Simulation
					}
					evaluator, err := NewModuleEvaluator(parsed.Modules[0], NewRNG(seed), fixture.Metadata)
					if err != nil {
						t.Fatal(err)
					}
					if err = evaluator.ObjectHashState().Restore(want.InitialUID); err != nil {
						t.Fatal(err)
					}
					recorder := NewTraceRecorder(0, evaluator.RNG)
					ctx := WithExecutionListener(context.Background(), recorder)
					ctx = WithRuntimeNondetOptions(ctx, RuntimeNondetOptions{RetrySmallerThan: Integer{decimal: "100"}})
					if mode == "simulation" {
						got, err := evaluator.Simulate(ctx, SimulationOptions{Init: "init", Step: "step", Invariant: "inv", MaxSamples: fixture.MaxSamples, MaxSteps: fixture.MaxSteps, NumberOfTraces: 1})
						assertSymbolicTraceException(t, err, want.Thrown)
						if want.Thrown == nil {
							if want.Outcome == nil || got.Status != want.Outcome.Status || got.Samples != want.Outcome.Samples || symbolicTraceJSON(t, got.Errors) != symbolicTraceJSON(t, want.Outcome.Errors) || len(got.BestTraces) != len(want.Outcome.BestTraces) {
								t.Fatal("simulation outcome", got, want.Outcome)
							}
							for i, trace := range got.BestTraces {
								if trace.Result != want.Outcome.BestTraces[i].Result || trace.Seed != want.Outcome.BestTraces[i].Seed || !reflect.DeepEqual(evaluator.Storage.VarNames(), want.Events[i].Vars) {
									t.Fatal("selected trace", trace, want.Outcome.BestTraces[i])
								}
								assertSymbolicTraceITF(t, want.Events[i], trace.SerializedStates, fixture.Metadata)
							}
						}
					} else {
						got, err := evaluator.Test(ctx, "example", fixture.MaxSamples)
						assertSymbolicTraceException(t, err, want.Thrown)
						if want.Thrown == nil {
							if want.Result == nil || got.Name != want.Result.Name || got.Status != want.Result.Status || got.Samples != want.Result.NSamples || got.Seed != want.Result.Seed || symbolicTraceJSON(t, got.Errors) != symbolicTraceJSON(t, want.Result.Errors) {
								t.Fatal("test result", got, want.Result)
							}
							if len(want.Events) > 0 {
								if !reflect.DeepEqual(evaluator.Storage.VarNames(), want.Events[0].Vars) {
									t.Fatal("variable order", evaluator.Storage.VarNames(), want.Events[0].Vars)
								}
								assertSymbolicTraceITF(t, want.Events[0], got.SerializedStates, fixture.Metadata)
							}
						}
					}
					if evaluator.ObjectHashState().Counter() != want.FinalUID || evaluator.RNG.State().String() != want.RNG || len(mustStates(&evaluator.Trace)) != want.TraceLength {
						t.Fatal("execution history", evaluator.ObjectHashState().Counter(), want.FinalUID, evaluator.RNG.State(), want.RNG, len(mustStates(&evaluator.Trace)), want.TraceLength)
					}
					best := recorder.BestTraces()
					if len(best) > 0 && len(best[0].Frame.Args) != want.BestTraceLength {
						t.Fatal("retained best trace length", len(best[0].Frame.Args), want.BestTraceLength)
					}
				})
			}
		})
	}
}

func TestUpstreamSymbolicSimulationCombinatorSnapshots(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Combinators []struct {
			Name, Source, RNG    string
			Input                Expr
			Metadata             bool
			Output               *Expr
			Error                *Diagnostic
			Thrown               *symbolicNondetException
			InitialUID, FinalUID uint32
			TraceLength          int
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_trace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Combinators {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			modules, err := ParseModules(fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			linked, err := ResolveLinkedModules(modules)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			analysis, err := AnalyzeModules(modules, linked.Table)
			if err != nil || len(analysis.Diagnostics) != 0 {
				t.Fatal("combinator static checking", err, analysis.Diagnostics)
			}
			evaluator, err := NewModuleEvaluator(modules[0], nil, fixture.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			if err = evaluator.ObjectHashState().Restore(fixture.InitialUID); err != nil {
				t.Fatal(err)
			}
			value, err := evaluator.EvaluateExpression(context.Background(), fixture.Input)
			assertSymbolicTraceException(t, err, fixture.Thrown)
			if fixture.Thrown == nil && (fixture.Output == nil || symbolicTraceJSON(t, value.Expression()) != symbolicTraceJSON(t, fixture.Output)) {
				t.Fatal("combinator output", value.String(), fixture.Output, err)
			}
			if evaluator.ObjectHashState().Counter() != fixture.FinalUID || evaluator.RNG.State().String() != fixture.RNG || len(mustStates(&evaluator.Trace)) != fixture.TraceLength {
				t.Fatal("combinator history", evaluator.ObjectHashState().Counter(), fixture.FinalUID, evaluator.RNG.State(), fixture.RNG, len(mustStates(&evaluator.Trace)), fixture.TraceLength)
			}
		})
	}
}
