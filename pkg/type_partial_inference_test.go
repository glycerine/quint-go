package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Upstream keeps provisional schemes when the definition's outer expression
// has a type, even if solving discovers a failure in one of its descendants.
func TestUpstreamPartialTypeInference(t *testing.T) {
	t.Parallel()
	type typeEntry struct {
		ID     string
		Scheme TypeScheme
	}
	type errorEntry struct {
		ID    string
		Error ErrorTree
	}
	var fixtures []struct {
		Name   string
		Input  []Module
		Types  []typeEntry
		Errors []errorEntry
	}
	data, err := os.ReadFile("testdata/parser/type_partial_inference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(fixture.Input)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			inferrer := NewTypeInferrer(linked.Table, nil)
			result, err := inferrer.InferTypes(fixture.Input[0].Declarations)
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
			gotTypes := []typeEntry{}
			for id, scheme := range result.Types.All() {
				gotTypes = append(gotTypes, typeEntry{ID: id, Scheme: withoutTypeIDs(scheme)})
			}
			for index := range fixture.Types {
				fixture.Types[index].Scheme = withoutTypeIDs(fixture.Types[index].Scheme)
			}
			if !reflect.DeepEqual(gotTypes, fixture.Types) {
				got, _ := json.Marshal(gotTypes)
				want, _ := json.Marshal(fixture.Types)
				t.Errorf("partial types Go %s\nTypeScript %s", got, want)
			}
			gotErrors := []errorEntry{}
			for id, failure := range result.Errors.All() {
				gotErrors = append(gotErrors, errorEntry{ID: id, Error: failure})
			}
			if !reflect.DeepEqual(gotErrors, fixture.Errors) {
				got, _ := json.Marshal(gotErrors)
				want, _ := json.Marshal(fixture.Errors)
				t.Errorf("errors Go %s\nTypeScript %s", got, want)
			}
			for range inferrer.Scope.All() {
				t.Fatal("failed definition leaked temporary types into declaration scope")
			}
		})
	}
}
