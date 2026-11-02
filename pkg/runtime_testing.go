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
}

// Test runs a named definition up to maxSamples times. A successful sample that
// does not advance the RNG needs no repetition. False and runtime-error results
// fail immediately; non-Boolean results are ignored. Every completed evaluation
// commits its pending state, including false/error/ignored results. Resets clear
// state and trace between samples without rewinding the RNG. Zero samples clears
// the trace and reports success without evaluating or resetting registers.
// Cancellation and invalid arguments return a Go error; runtime diagnostics are
// returned in the result. Callers must use this evaluator sequentially.
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
	e.Trace.Reset()
	bestLength := -1
	for out.Samples < maxSamples {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out.Seed = e.RNG.State()
		e.Reset()
		value, evaluationError := e.Evaluate(ctx, name, nil)
		if err := ctx.Err(); err != nil {
			return out, err
		}
		shiftError := e.Shift()
		out.Samples++
		if evaluationError == nil {
			evaluationError = shiftError
		}
		failed := evaluationError != nil || (value.expression.Kind == "bool" && !value.expression.Bool)
		states := e.Trace.States()
		if failed || len(states) > bestLength {
			out.States = states
			bestLength = len(states)
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
		if value.expression.Kind != "bool" {
			out.Status = "ignored"
			return out, nil
		}
		if !value.expression.Bool {
			id := definition.ID
			out.Status = "failed"
			out.Errors = append(out.Errors, Diagnostic{Code: "QNT511", Message: "Test " + name + " returned false", Reference: &id})
			return out, nil
		}
		if e.RNG.State() == out.Seed {
			return out, nil
		}
	}
	return out, nil
}
