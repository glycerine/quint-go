package quint

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamRecursiveInitConversion(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source string
		RemoveRuns   bool
		Input        Module
		Output       *Module
		Errors       []Diagnostic
		Modes        []struct {
			ID        string
			Qualifier Qualifier
		}
	}
	data, err := os.ReadFile("testdata/parser/convert_init_recursive.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 12 {
		t.Fatalf("got %d recursive conversion fixtures, want 12", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules([]Module{fixture.Input})
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			modes := insdict.NewDict[string, Qualifier]()
			for _, mode := range fixture.Modes {
				modes.Set(mode.ID, mode.Qualifier)
			}
			before, err := json.Marshal(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			modeBefore, err := json.Marshal(modes)
			if err != nil {
				t.Fatal(err)
			}
			input := fixture.Input
			if fixture.RemoveRuns {
				input = RemoveRunDefinitions(input)
			}
			output, diagnostics := ConvertInit(input, linked.Table, modes)
			if len(diagnostics) != len(fixture.Errors) {
				t.Fatalf("Go diagnostics %+v; upstream %+v", diagnostics, fixture.Errors)
			}
			if len(fixture.Errors) != 0 {
				assertRecursiveInitJSON(t, diagnostics, fixture.Errors)
			} else {
				assertRecursiveInitJSON(t, output, fixture.Output)
				// All expression containers modified by conversion are detached.
				// Mutating the result must not alter even deeply nested input IR.
				for _, declaration := range output.Declarations {
					if declaration.Expr != nil {
						var expressions []*Expr
						if err := WalkExpr(declaration.Expr, func(expression *Expr) (bool, error) {
							expressions = append(expressions, expression)
							return true, nil
						}); err != nil {
							t.Fatal(err)
						}
						for _, expression := range expressions {
							expression.Name = "changed"
							expression.Opcode = "changed"
						}
					}
				}
			}
			after, err := json.Marshal(fixture.Input)
			if err != nil || string(after) != string(before) {
				t.Fatalf("conversion changed its source input: %v", err)
			}
			modeAfter, err := json.Marshal(modes)
			if err != nil || string(modeAfter) != string(modeBefore) {
				t.Fatalf("conversion changed inferred modes: %v", err)
			}
		})
	}
}

func assertRecursiveInitJSON(t *testing.T, got, want any) {
	t.Helper()
	actual, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Fatalf("Go %s\nupstream %s", actual, expected)
	}
}
