package quint

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
)

// SimulationOptions names definitions in an already prepared module. Limits are
// explicit: zero samples or zero error traces performs no runs; zero steps still
// evaluates initialization and its invariant. Disabled steps end a sample without
// claiming a deadlock. Invariant is required, including for an always-true check.
type SimulationOptions struct {
	Init           string   `json:"init"`
	Step           string   `json:"step"`
	Invariant      string   `json:"invariant"`
	Witnesses      []string `json:"witnesses"`
	MaxSamples     int      `json:"maxSamples"`
	MaxSteps       int      `json:"maxSteps"`
	NumberOfTraces int      `json:"numberOfTraces"`
}

type SimulationTrace struct {
	States []RuntimeValue
	Result bool
	Seed   Integer
}

type SimulationTraceStatistics struct {
	AverageTraceLength float64 `json:"averageTraceLength"`
	MinTraceLength     float64 `json:"minTraceLength"`
	MaxTraceLength     float64 `json:"maxTraceLength"`
}

type SimulationOutcome struct {
	Status             string
	Errors             []Diagnostic
	BestTraces         []SimulationTrace
	WitnessingTraces   []int
	Samples            int
	TraceStatistics    SimulationTraceStatistics
	ViolatedInvariants []int
}

// Reset clears execution registers and trace without rewinding the RNG.
func (e *ModuleEvaluator) Reset() { e.Trace.Reset(); e.Storage.Reset() }

// Simulate reproduces the sequential TypeScript simulation policy. Runtime
// diagnostics are returned in the outcome; cancellation and invalid API options
// return an error. The evaluator retains its last state/trace and advanced RNG.
// Witnesses are checked after successful steps only; their runtime errors are
// ignored. Each witness counts at most once per sample.
func (e *ModuleEvaluator) Simulate(ctx context.Context, options SimulationOptions) (SimulationOutcome, error) {
	return e.simulate(ctx, options, nil)
}

func (e *ModuleEvaluator) simulate(ctx context.Context, options SimulationOptions, evaluateValue func(string) (RuntimeValue, error)) (SimulationOutcome, error) {
	out := SimulationOutcome{Status: "ok", Errors: []Diagnostic{}, BestTraces: []SimulationTrace{}, WitnessingTraces: make([]int, len(options.Witnesses)), ViolatedInvariants: []int{}, TraceStatistics: SimulationTraceStatistics{AverageTraceLength: math.NaN(), MinTraceLength: math.Inf(1)}}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if options.MaxSamples < 0 || options.MaxSteps < 0 || options.NumberOfTraces < 0 {
		return out, fmt.Errorf("simulation limits must be nonnegative")
	}
	if evaluateValue == nil {
		selected := append([]string{options.Init, options.Step, options.Invariant}, options.Witnesses...)
		for _, name := range selected {
			if _, found := e.definitions.Get2(name); !found {
				return out, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name}
			}
		}
		for _, name := range selected {
			if err := e.PrepareDefinition(ctx, name); err != nil {
				return out, err
			}
		}
		evaluateValue = func(name string) (RuntimeValue, error) { return e.Evaluate(ctx, name, nil) }
	}
	type rankedTrace struct {
		trace   SimulationTrace
		failure *Diagnostic
	}
	best := []rankedTrace{}
	var failure *Diagnostic
	listener := ExecutionListenerFromContext(ctx)
	var initApp, stepApp Expr
	if listener != nil {
		initApp = Expr{Kind: "app", Opcode: "q::initAndInvariant", Args: []Expr{}}
		stepApp = Expr{Kind: "app", Opcode: "q::stepAndInvariant", Args: []Expr{}}
	}
	errorsFound := 0
	lengths := []int{}
	var interrupted error
	var lastValue RuntimeValue
	var lastError error
	evaluate := func(name string, ignoreFailure bool) bool {
		value, err := evaluateValue(name)
		lastValue, lastError = value, err
		if err != nil {
			if ctx.Err() != nil {
				interrupted = ctx.Err()
				return false
			}
			if !ignoreFailure {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) {
					diagnostic = Diagnostic{Code: "QNT501", Message: err.Error()}
				}
				failure = &diagnostic
			}
			return false
		}
		if value.expression.Kind != "bool" {
			if !ignoreFailure {
				failure = &Diagnostic{Code: "QNT501", Message: "Simulation expression must return a Boolean value"}
			}
			return false
		}
		return value.expression.Bool
	}
	shift := func() bool {
		if err := e.Shift(); err != nil {
			lastValue, lastError = RuntimeValue{}, err
			var diagnostic Diagnostic
			if !errors.As(err, &diagnostic) {
				diagnostic = Diagnostic{Code: "QNT501", Message: err.Error()}
			}
			failure = &diagnostic
			return false
		}
		return true
	}
	for out.Samples < options.MaxSamples && errorsFound < options.NumberOfTraces && failure == nil {
		if err := ctx.Err(); err != nil {
			interrupted = err
			break
		}
		seed := e.RNG.State()
		if listener != nil {
			listener.OnRunCall()
		}
		e.Reset()
		witnessed := make([]bool, len(options.Witnesses))
		if listener != nil {
			listener.OnUserOperatorCall(initApp)
		}
		if !evaluate(options.Init, false) {
			lengths = append(lengths, 0)
			if listener != nil {
				listener.OnUserOperatorReturn(initApp, nil, lastValue, lastError)
			}
		} else if shift() {
			invariantHolds := evaluate(options.Invariant, false)
			if listener != nil {
				listener.OnUserOperatorReturn(initApp, nil, lastValue, lastError)
			}
			if !invariantHolds {
				errorsFound++
				lengths = append(lengths, len(e.Trace.states))
			} else {
				for step := 0; step < options.MaxSteps && errorsFound < options.NumberOfTraces && failure == nil && interrupted == nil; step++ {
					if listener != nil {
						listener.OnUserOperatorCall(stepApp)
					}
					if !evaluate(options.Step, false) {
						if listener != nil {
							listener.OnUserOperatorReturn(stepApp, nil, lastValue, lastError)
						}
						break
					}
					if !shift() {
						if listener != nil {
							listener.OnUserOperatorReturn(stepApp, nil, lastValue, lastError)
						}
						break
					}
					for i, name := range options.Witnesses {
						if evaluate(name, true) {
							witnessed[i] = true
						}
					}
					if !evaluate(options.Invariant, false) {
						errorsFound++
					}
					if listener != nil {
						listener.OnUserOperatorReturn(stepApp, nil, lastValue, lastError)
					}
				}
				for i, value := range witnessed {
					if value {
						out.WitnessingTraces[i]++
					}
				}
				lengths = append(lengths, len(e.Trace.states))
			}
		} else if listener != nil {
			listener.OnUserOperatorReturn(initApp, nil, lastValue, lastError)
		}
		if interrupted != nil {
			if listener != nil {
				listener.OnRunReturn(RuntimeValue{}, interrupted, e.Trace.States())
			}
			break
		}
		trace := rankedTrace{trace: SimulationTrace{States: e.Trace.States(), Result: errorsFound == 0, Seed: seed}, failure: failure}
		if listener != nil {
			if failure != nil {
				listener.OnRunReturn(RuntimeValue{}, *failure, trace.trace.States)
			} else {
				result, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: errorsFound == 0})
				listener.OnRunReturn(result, nil, trace.trace.States)
			}
		}
		// Insertion after existing equal-quality traces preserves encounter order.
		bad := func(t rankedTrace) bool { return t.failure != nil || !t.trace.Result }
		index := sort.Search(len(best), func(i int) bool {
			if bad(best[i]) != bad(trace) {
				return !bad(best[i])
			}
			if bad(trace) {
				return len(best[i].trace.States) > len(trace.trace.States)
			}
			return len(best[i].trace.States) < len(trace.trace.States)
		})
		best = append(best, rankedTrace{})
		copy(best[index+1:], best[index:])
		best[index] = trace
		if len(best) > options.NumberOfTraces {
			best = best[:options.NumberOfTraces]
		}
		out.Samples++
	}
	if len(lengths) > 0 {
		total := 0
		for _, length := range lengths {
			total += length
			out.TraceStatistics.MinTraceLength = math.Min(out.TraceStatistics.MinTraceLength, float64(length))
			out.TraceStatistics.MaxTraceLength = math.Max(out.TraceStatistics.MaxTraceLength, float64(length))
		}
		out.TraceStatistics.AverageTraceLength = float64(total) / float64(len(lengths))
	}
	if failure != nil {
		out.Status = "error"
		out.Errors = append(out.Errors, *failure)
	} else if errorsFound > 0 {
		out.Status = "violation"
	}
	selectedErrors := []Diagnostic{}
	for _, trace := range best {
		if trace.failure != nil {
			selectedErrors = append(selectedErrors, *trace.failure)
		}
	}
	if len(selectedErrors) > 0 {
		out.Errors = append(out.Errors, selectedErrors...)
	} else {
		for _, trace := range best {
			out.BestTraces = append(out.BestTraces, trace.trace)
		}
	}
	if interrupted != nil {
		out.Status = "error"
		return out, interrupted
	}
	return out, nil
}
