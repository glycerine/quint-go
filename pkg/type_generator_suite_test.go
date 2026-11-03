package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamTypeConstraintGeneratorSuite(t *testing.T) {
	t.Parallel()
	type failure struct {
		ID    string
		Error ErrorTree
	}
	type entry struct {
		ID     string
		Scheme TypeScheme
	}
	var cases []struct {
		Name  string
		Input []Module
		Calls []struct {
			Input         Constraint
			Substitutions []typeCompositionSubstitution
			Errors        []failure
		}
		Types  []entry
		Errors []failure
	}
	data, err := os.ReadFile("testdata/parser/type_generator_suite.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 12 {
		t.Fatal("missing upstream generator tests")
	}
	for _, f := range cases {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			linked, err := ResolveLinkedModules(f.Input)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			g := NewTypeConstraintGenerator(TypeUnifier{Table: linked.Table})
			call := 0
			g.Solver = func(input Constraint) (ConstraintSolution, error) {
				if call >= len(f.Calls) {
					t.Fatal("unexpected solving request")
				}
				want := f.Calls[call]
				call++
				if !reflect.DeepEqual(input, want.Input) {
					t.Fatalf("Go solver request %s (%+v)\nTypeScript %s (%+v)", ConstraintString(input), input, ConstraintString(want.Input), want.Input)
				}
				solution := ConstraintSolution{Substitutions: decodeTypeCompositionSubstitutions(t, want.Substitutions)}
				if want.Errors != nil {
					solution.Errors = insdict.NewDict[string, ErrorTree]()
					for _, failure := range want.Errors {
						solution.Errors.Set(failure.ID, failure.Error)
					}
				}
				return solution, nil
			}
			for _, module := range f.Input {
				for _, declaration := range module.Declarations {
					if err := g.GenerateDeclaration(declaration); err != nil {
						t.Fatal(err)
					}
				}
			}
			if call != len(f.Calls) {
				t.Fatal("missing solving requests", call, len(f.Calls))
			}
			types, failures := []entry{}, []failure{}
			for id, scheme := range g.Context.Types.All() {
				types = append(types, entry{id, scheme})
			}
			for id, err := range g.Context.Errors.All() {
				failures = append(failures, failure{id, err})
			}
			if !reflect.DeepEqual(types, f.Types) {
				got, _ := json.Marshal(types)
				want, _ := json.Marshal(f.Types)
				t.Errorf("Go types %s\nTypeScript %s", got, want)
			}
			if !reflect.DeepEqual(failures, f.Errors) {
				t.Errorf("Go errors %+v\nTypeScript %+v", failures, f.Errors)
			}
		})
	}
}
