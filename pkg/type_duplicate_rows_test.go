package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamDuplicateRowTypes(t *testing.T) {
	t.Parallel()
	type entry struct {
		ID     string
		Scheme TypeScheme
	}
	type failure struct {
		ID    string
		Error ErrorTree
	}
	var fixtures struct {
		Inference []struct {
			Name   string
			Input  []Module
			Types  []entry
			Errors []failure
		}
		Unification []struct {
			Name          string
			Input         [2]Type
			Error         *ErrorTree
			Substitutions []struct {
				Kind, Name string
				Value      json.RawMessage
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/type_duplicate_rows.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Inference {
		t.Run(fixture.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(fixture.Input)
			if err != nil || len(linked.Errors) > 0 {
				t.Fatal(err, linked.Errors)
			}
			result, err := NewTypeInferrer(linked.Table, nil).InferTypes(fixture.Input[0].Declarations)
			if err != nil {
				t.Fatal(err)
			}
			withoutTypeIDs := func(scheme TypeScheme) TypeScheme {
				t.Helper()
				typ, err := ApplySubstitution(nil, scheme.Type)
				if err != nil {
					t.Fatal(err)
				}
				_ = WalkType(&typ, func(typ *Type) (bool, error) { typ.ID = nil; return true, nil })
				scheme.Type = typ
				return scheme
			}
			got := []entry{}
			for id, scheme := range result.Types.All() {
				got = append(got, entry{id, withoutTypeIDs(scheme)})
			}
			for index := range fixture.Types {
				fixture.Types[index].Scheme = withoutTypeIDs(fixture.Types[index].Scheme)
			}
			if !reflect.DeepEqual(got, fixture.Types) {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(fixture.Types)
				t.Errorf("types Go %s\nTypeScript %s", a, b)
			}
			errors := []failure{}
			for id, error := range result.Errors.All() {
				errors = append(errors, failure{id, error})
			}
			if !reflect.DeepEqual(errors, fixture.Errors) {
				t.Errorf("errors Go %+v\nTypeScript %+v", errors, fixture.Errors)
			}
		})
	}
	for _, fixture := range fixtures.Unification {
		t.Run(fixture.Name, func(t *testing.T) {
			before, _ := json.Marshal(fixture.Input)
			got, failure := (TypeUnifier{}).Unify(fixture.Input[0], fixture.Input[1])
			if !reflect.DeepEqual(failure, fixture.Error) {
				t.Fatal("error", failure, fixture.Error)
			}
			var want []Substitution
			if fixture.Substitutions != nil {
				want = []Substitution{}
			}
			for _, input := range fixture.Substitutions {
				substitution := Substitution{Kind: input.Kind, Name: input.Name}
				if input.Kind == "type" {
					err = json.Unmarshal(input.Value, &substitution.Type)
				} else {
					err = json.Unmarshal(input.Value, &substitution.Row)
				}
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, substitution)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("substitutions Go %+v\nTypeScript %+v", got, want)
			}
			after, _ := json.Marshal(fixture.Input)
			if string(before) != string(after) {
				t.Fatal("unification mutated its input row trees")
			}
		})
	}
}
