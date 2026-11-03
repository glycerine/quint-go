package quint

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamStateSnapshotAliasing(t *testing.T) {
	t.Parallel()
	type namedValue struct {
		Name  string
		Value *Expr
	}
	var fixtures []struct {
		Name, Source, Classification string
		AnalysisErrors               []Diagnostic
		Module                       Module
		Operations                   []struct {
			Operation, RNG string
			Output, Record Expr
			Picks, Next    []namedValue
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_state_snapshot_alias.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 8 {
		t.Fatalf("snapshot alias oracle has %d cases, want 8", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			if len(fixture.AnalysisErrors) != 0 {
				t.Fatalf("upstream analysis returned diagnostics: %v", fixture.AnalysisErrors)
			}
			if fixture.Name == "empty_any_preserves" {
				if fixture.Classification != "internal-nullary-actionAny" {
					t.Fatal("nullary internal operator lost explicit classification")
				}
			} else if fixture.Classification != "typed-source" {
				t.Fatalf("fixture is not analyzed typed source: %s %v", fixture.Classification, fixture.AnalysisErrors)
			}
			evaluator, err := NewModuleEvaluator(fixture.Module, NewRNG(Integer{}), true)
			if err != nil {
				t.Fatal(err)
			}
			assertValue := func(label string, value RuntimeValue, want *Expr) {
				t.Helper()
				if want == nil {
					if value.Valid() {
						t.Fatalf("%s: expected unset, got %s", label, value.String())
					}
					return
				}
				expected, err := NewRuntimeValue(*want)
				if err != nil || !value.Equal(expected) {
					t.Fatalf("%s: got %s, want %s (%v)", label, value.String(), ExpressionString(*want), err)
				}
			}
			for _, operation := range fixture.Operations {
				value, err := evaluator.Evaluate(context.Background(), operation.Operation, nil)
				if err != nil {
					t.Fatalf("%s: %v", operation.Operation, err)
				}
				assertValue(operation.Operation+" output", value, &operation.Output)
				record, err := evaluator.Storage.Record()
				if err != nil {
					t.Fatal(err)
				}
				assertValue(operation.Operation+" record", record, &operation.Record)
				if evaluator.RNG.State().String() != operation.RNG {
					t.Fatalf("%s RNG %s, want %s", operation.Operation, evaluator.RNG.State(), operation.RNG)
				}
				if evaluator.Storage.NondetPicks.Len() != len(operation.Picks) {
					t.Fatalf("%s pick count %d, want %d", operation.Operation, evaluator.Storage.NondetPicks.Len(), len(operation.Picks))
				}
				for _, pick := range operation.Picks {
					value, exists := evaluator.Storage.NondetPicks.Get2(pick.Name)
					if !exists {
						t.Fatalf("missing registered pick %s", pick.Name)
					}
					assertValue(operation.Operation+" pick "+pick.Name, value, pick.Value)
				}
				if evaluator.Storage.next.Len() != len(operation.Next) {
					t.Fatalf("%s next register count mismatch", operation.Operation)
				}
				for _, expected := range operation.Next {
					found := false
					for _, register := range evaluator.Storage.next.All() {
						if register.Name == expected.Name {
							found = true
							assertValue(operation.Operation+" next "+expected.Name, register.Value, expected.Value)
							break
						}
					}
					if !found {
						t.Fatalf("missing next register %s", expected.Name)
					}
				}
			}
		})
	}
}
