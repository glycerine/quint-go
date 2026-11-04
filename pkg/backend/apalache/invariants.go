package apalache

import (
	"context"
	"encoding/json"
	"fmt"

	quint "github.com/glycerine/quint-go/pkg"
)

// FindViolatedInvariants evaluates separately compiled ordinary predicates at
// the final ITF state. Compile with InvariantReports enabled and pass the same
// labels in compilation order. An error affects reporting only; this helper does
// not change the checker's verdict or interpret temporal counterexamples.
func FindViolatedInvariants(ctx context.Context, compiled quint.CompilationResult, labels []string, traceJSON json.RawMessage) ([]int, error) {
	violated := []int{}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(labels) == 0 {
		return violated, nil
	}
	trace, err := quint.DecodeITFTrace(traceJSON)
	if err != nil {
		return nil, fmt.Errorf("decode invariant-report trace: %w", err)
	}
	if len(trace.States) == 0 {
		return nil, fmt.Errorf("invariant-report trace has no states")
	}
	state, err := quint.NewRuntimeValue(trace.States[len(trace.States)-1])
	if err != nil {
		return nil, fmt.Errorf("decode invariant-report state: %w", err)
	}
	// Checker variables use the flattened transport names, whereas in-memory
	// source evaluation can carry private import/instance display provenance.
	// The JSON boundary deliberately strips that provenance without altering
	// the caller's compilation or depending on private runtime fields.
	encoded, err := json.Marshal(compiled.MainModule)
	if err != nil {
		return nil, fmt.Errorf("encode invariant-report module: %w", err)
	}
	var module quint.Module
	if err := json.Unmarshal(encoded, &module); err != nil {
		return nil, fmt.Errorf("decode invariant-report module: %w", err)
	}
	evaluator, err := quint.NewModuleEvaluator(module, nil, false)
	if err != nil {
		return nil, err
	}
	// Prepare every predicate first so lazy variable discovery completes before
	// FromRecord restores values, including variables used only by later checks.
	reportName := func(i int) string {
		if i < len(compiled.InvariantReportNames) && compiled.InvariantReportNames[i] != "" {
			return compiled.InvariantReportNames[i]
		}
		return quint.InvariantReportName(i)
	}
	for i := range labels {
		if err := evaluator.PrepareDefinition(ctx, reportName(i)); err != nil {
			return nil, fmt.Errorf("prepare invariant %q: %w", labels[i], err)
		}
	}
	if err := evaluator.Storage.FromRecord(state); err != nil {
		return nil, fmt.Errorf("restore invariant-report state: %w", err)
	}
	for i, label := range labels {
		value, err := evaluator.Evaluate(ctx, reportName(i), nil)
		if err != nil {
			return nil, fmt.Errorf("evaluate invariant %q: %w", label, err)
		}
		expression := value.Expression()
		if expression.Kind != "bool" {
			return nil, fmt.Errorf("invariant %q did not evaluate to a Boolean", label)
		}
		if !expression.Bool {
			violated = append(violated, i)
		}
	}
	return violated, nil
}
