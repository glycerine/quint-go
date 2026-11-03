package quint

import (
	"context"
	"errors"
	"fmt"
)

// RuntimeTestResult reports one named test. Seed is the seed of the final
// attempted sample, suitable for replay. States contains the selected best
// state trace: failures take precedence, otherwise longer traces win and ties
// retain the earlier sample. Execution frames are not part of this API.
type RuntimeTestResult struct {
	Name    string
	Status  string
	Errors  []Diagnostic
	Seed    Integer
	Samples int
	States  []RuntimeValue
	// SerializedStates is the checked snapshot taken at the reference runtime's
	// result boundary. Reuse it when exporting a trace to preserve display order
	// and the evaluator's object-hash history.
	SerializedStates []Expr
}

// Test runs a named definition up to maxSamples times. A successful sample that
// does not advance the RNG needs no repetition. False and runtime-error results
// fail immediately; non-Boolean results are ignored. Every completed evaluation
// commits its pending state, including false/error/ignored results. Resets clear
// state and trace between samples without rewinding the RNG. Zero samples clears
// the trace and reports success without evaluating or resetting registers.
// Cancellation, invalid arguments, and display serialization exceptions return
// a Go error; runtime diagnostics are returned in the result. Callers must use
// this evaluator sequentially.
func (e *ModuleEvaluator) Test(ctx context.Context, name string, maxSamples int) (RuntimeTestResult, error) {
	out := RuntimeTestResult{Name: name, Status: "passed", Errors: []Diagnostic{}, Seed: e.RNG.State()}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if maxSamples < 0 {
		return out, fmt.Errorf("test sample limit must be nonnegative")
	}
	definition, found := e.definitions.Get2(name)
	if !found {
		return out, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name}
	}
	if err := e.PrepareDefinition(ctx, name); err != nil {
		return out, err
	}
	e.Trace.Reset()
	listener := ExecutionListenerFromContext(ctx)
	if recorder, ok := listener.(interface{ Clear() }); ok {
		recorder.Clear()
	}
	bestLength := -1
	for out.Samples < maxSamples {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out.Seed = e.RNG.State()
		if listener != nil {
			listener.OnRunCall()
		}
		e.Reset()
		value, evaluationError := e.Evaluate(ctx, name, nil)
		if err := ctx.Err(); err != nil {
			if listener != nil {
				listener.OnRunReturn(value, err, e.Trace.States())
			}
			return out, err
		}
		shiftError := e.Shift()
		out.Samples++
		if evaluationError == nil {
			evaluationError = shiftError
		}
		failed := evaluationError != nil || (value.expression.Kind == "bool" && !value.expression.Bool)
		states := e.Trace.States()
		if listener != nil {
			listener.OnRunReturn(value, evaluationError, states)
		}
		if failed || len(states) > bestLength {
			out.States = states
			bestLength = len(states)
		}
		// The reference serializes the selected trace before classifying the
		// result, including runtime diagnostics and ignored non-Boolean values.
		var err error
		out.SerializedStates, err = SerializeRuntimeTraceContext(ctx, out.States, e.ObjectHashState())
		if err != nil {
			return out, err
		}
		if evaluationError != nil {
			var diagnostic Diagnostic
			if !errors.As(evaluationError, &diagnostic) {
				diagnostic = Diagnostic{Code: "QNT501", Message: evaluationError.Error()}
			}
			out.Status = "failed"
			out.Errors = append(out.Errors, diagnostic)
			return out, nil
		}
		// Malformed results are classified from their display expression too;
		// a compact result can fail serialization before it is ignored.
		expression, err := SerializeRuntimeValueContext(ctx, value, e.ObjectHashState())
		if err != nil {
			return out, err
		}
		if expression.Kind != "bool" {
			out.Status = "ignored"
			return out, nil
		}
		if !expression.Bool {
			id := definition.ID
			out.Status = "failed"
			out.Errors = append(out.Errors, Diagnostic{Code: "QNT511", Message: "Test " + name + " returned false", Reference: &id})
			return out, nil
		}
		if e.RNG.State() == out.Seed {
			return out, nil
		}
	}
	// Exhausting the sample limit takes one further snapshot before onTrace.
	var err error
	out.SerializedStates, err = SerializeRuntimeTraceContext(ctx, out.States, e.ObjectHashState())
	if err != nil {
		return out, err
	}
	return out, nil
}
