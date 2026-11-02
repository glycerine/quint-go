package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamTypeInference(t *testing.T) {
	type entry struct {
		ID     string
		Scheme TypeScheme
	}
	type failure struct {
		ID    string
		Error ErrorTree
	}
	var fixtures []struct {
		Name   string
		Input  []Module
		Types  []entry
		Errors []failure
	}
	data, err := os.ReadFile("testdata/parser/inference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			g := NewTypeConstraintGenerator(TypeUnifier{Table: linked.Table})
			for _, d := range f.Input[0].Declarations {
				if err = g.GenerateDeclaration(d); err != nil {
					t.Fatal(err)
				}
			}
			got := []entry{}
			for id, s := range g.Context.Types.All() {
				s.Type = SimplifyType(s.Type)
				_ = WalkType(&s.Type, func(t *Type) (bool, error) { t.ID = nil; return true, nil })
				got = append(got, entry{id, s})
			}
			for i := range f.Types {
				_ = WalkType(&f.Types[i].Scheme.Type, func(t *Type) (bool, error) { t.ID = nil; return true, nil })
			}
			if !reflect.DeepEqual(got, f.Types) {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(f.Types)
				t.Errorf("Go %s\nTS %s", a, b)
			}
			errors := []failure{}
			for id, e := range g.Context.Errors.All() {
				errors = append(errors, failure{id, e})
			}
			if !reflect.DeepEqual(errors, f.Errors) {
				t.Errorf("Go %+v\nTS %+v", errors, f.Errors)
			}
		})
	}
}
