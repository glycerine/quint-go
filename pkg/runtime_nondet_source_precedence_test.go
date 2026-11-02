package quint

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamTypedNondetPrecedence(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Seed, RNG string
		AnalysisErrors          []Diagnostic
		Output                  *Expr
		Error                   *Diagnostic
		Thrown                  *symbolicNondetException
		Picks, Pending          []nondetPickSnapshot
	}
	data, err := os.ReadFile("testdata/parser/runtime_nondet_source_precedence.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
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
			if err != nil {
				t.Fatal(err)
			}
			if len(analysis.Diagnostics) != len(fixture.AnalysisErrors) {
				t.Fatal("static diagnostics", analysis.Diagnostics, fixture.AnalysisErrors)
			}
			for i, want := range fixture.AnalysisErrors {
				got := analysis.Diagnostics[i]
				if got.Code != want.Code || got.Message != want.Message || (got.Reference == nil) != (want.Reference == nil) || got.Reference != nil && *got.Reference != *want.Reference {
					t.Fatal("static diagnostic", got, want)
				}
			}
			if len(fixture.AnalysisErrors) != 0 {
				return
			}
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := NewModuleEvaluator(analysis.Modules[0], NewRNG(seed), true)
			if err != nil {
				t.Fatal(err)
			}
			ctx := WithRuntimeNondetOptions(context.Background(), RuntimeNondetOptions{RetrySmallerThan: Integer{decimal: "100"}})
			value, err := evaluator.Evaluate(ctx, "result", nil)
			assertSymbolicNondetOutcome(t, value, err, fixture.Output, fixture.Error, fixture.Thrown)
			assertNondetPicks(t, evaluator.Storage, fixture.Picks)
			pending := []nondetPickSnapshot{}
			for _, register := range evaluator.Storage.next.All() {
				if register.Value.Valid() {
					expression, err := SerializeRuntimeValueContext(ctx, register.Value, evaluator.ObjectHashState())
					if err != nil {
						t.Fatal(err)
					}
					pending = append(pending, nondetPickSnapshot{Name: register.Name, Value: &expression})
				}
			}
			if len(pending) != len(fixture.Pending) {
				t.Fatal("pending updates", pending, fixture.Pending)
			}
			for i, want := range fixture.Pending {
				got := pending[i]
				if got.Name != want.Name || ExpressionString(*got.Value) != ExpressionString(*want.Value) {
					t.Fatal("pending update", got, want)
				}
			}
			if evaluator.RNG.State().String() != fixture.RNG {
				t.Fatal("RNG state", evaluator.RNG.State(), fixture.RNG)
			}
		})
	}
}
