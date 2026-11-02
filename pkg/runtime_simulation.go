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
	out := SimulationOutcome{Status: "ok", Errors: []Diagnostic{}, BestTraces: []SimulationTrace{}, WitnessingTraces: make([]int, len(options.Witnesses)), ViolatedInvariants: []int{}, TraceStatistics: SimulationTraceStatistics{AverageTraceLength: math.NaN(), MinTraceLength: math.Inf(1)}}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if options.MaxSamples < 0 || options.MaxSteps < 0 || options.NumberOfTraces < 0 {
		return out, fmt.Errorf("simulation limits must be nonnegative")
	}
	for _, name := range append([]string{options.Init, options.Step, options.Invariant}, options.Witnesses...) {
		if _, ok := e.definitions.Get2(name); !ok {
			return out, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name}
		}
	}
	type rankedTrace struct {
		trace   SimulationTrace
		failure *Diagnostic
	}
	best := []rankedTrace{}
	var failure *Diagnostic
	errorsFound := 0
	lengths := []int{}
	var interrupted error
	evaluate := func(name string, ignoreFailure bool) bool {
		value, err := e.Evaluate(ctx, name, nil)
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
		e.Reset()
		witnessed := make([]bool, len(options.Witnesses))
		if !evaluate(options.Init, false) {
			lengths = append(lengths, 0)
		} else if shift() {
			if !evaluate(options.Invariant, false) {
				errorsFound++
				lengths = append(lengths, len(e.Trace.states))
			} else {
				for step := 0; step < options.MaxSteps && errorsFound < options.NumberOfTraces && failure == nil && interrupted == nil; step++ {
					if !evaluate(options.Step, false) {
						break
					}
					if !shift() {
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
				}
				for i, value := range witnessed {
					if value {
						out.WitnessingTraces[i]++
					}
				}
				lengths = append(lengths, len(e.Trace.states))
			}
		}
		if interrupted != nil {
			break
		}
		trace := rankedTrace{trace: SimulationTrace{States: e.Trace.States(), Result: errorsFound == 0, Seed: seed}, failure: failure}
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
