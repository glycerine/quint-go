package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamConstraintSolver(t *testing.T) {
	var fixtures []struct {
		Name          string
		Input         Constraint
		Substitutions []struct {
			Kind, Name string
			Value      json.RawMessage
		}
		Errors []struct {
			ID    string
			Error ErrorTree
		}
	}
	data, err := os.ReadFile("testdata/parser/constraint_solver.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			result, err := (TypeUnifier{}).SolveConstraint(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for id, e := range result.Errors.All() {
				if n >= len(f.Errors) || id != f.Errors[n].ID || !reflect.DeepEqual(e, f.Errors[n].Error) {
					t.Fatalf("error %s %+v expected %+v", id, e, f.Errors)
				}
				n++
			}
			if n != len(f.Errors) {
				t.Fatal("error count")
			}
			var want []Substitution
			if f.Substitutions != nil {
				want = []Substitution{}
			}
			for _, s := range f.Substitutions {
				sub := Substitution{Kind: s.Kind, Name: s.Name}
				if s.Kind == "type" {
					err = json.Unmarshal(s.Value, &sub.Type)
				} else {
					err = json.Unmarshal(s.Value, &sub.Row)
				}
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, sub)
			}
			if !reflect.DeepEqual(result.Substitutions, want) {
				a, _ := json.Marshal(result.Substitutions)
				b, _ := json.Marshal(want)
				t.Fatalf("Go %s TS %s", a, b)
			}
		})
	}
}
