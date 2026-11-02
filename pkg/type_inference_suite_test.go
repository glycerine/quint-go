package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Replay the entire upstream inference test suite without Node during Go tests.
// The focused inference fixtures cover the same paths in the default fast suite.
func TestUpstreamTypeInferenceSuite(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for the full upstream inference suite")
	}
	t.Parallel()
	type entry struct {
		ID     string
		Scheme TypeScheme
	}
	type failure struct {
		ID    string
		Error ErrorTree
	}
	var cases []struct {
		Name              string
		Input             []Module
		Resolved          []Declaration
		ApplicationErrors []failure
		Types             []entry
		Errors            []failure
	}
	data, err := os.ReadFile("testdata/parser/type_inference_suite.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, f := range cases {
		t.Run(f.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(f.Input)
			if err != nil || len(linked.Errors) > 0 {
				t.Fatal(err, linked.Errors)
			}
			apps := NewTypeApplicationResolver(linked.Table)
			if err = apps.ResolveLookupTable(); err != nil {
				t.Fatal(err)
			}
			resolved := []Declaration{}
			for _, m := range f.Input {
				decls, err := apps.ResolveDeclarations(m.Declarations)
				if err != nil {
					t.Fatal(err)
				}
				resolved = append(resolved, decls...)
			}
			appErrors := []failure{}
			for id, e := range apps.Errors.All() {
				appErrors = append(appErrors, failure{id, e})
			}
			a, _ := json.Marshal(resolved)
			b, _ := json.Marshal(f.Resolved)
			if string(a) != string(b) {
				t.Error("resolved declarations differ")
			}
			if !reflect.DeepEqual(appErrors, f.ApplicationErrors) {
				t.Errorf("app errors %+v want %+v", appErrors, f.ApplicationErrors)
			}
			infer := NewTypeInferrer(linked.Table, nil)
			result, err := infer.InferTypes(resolved)
			if err != nil {
				t.Fatal(err)
			}
			// Compare complete type-node provenance as well as scheme structure,
			// quantifiers, insertion order and diagnostics.
			got := []entry{}
			for id, s := range result.Types.All() {
				got = append(got, entry{id, s})
			}
			if !reflect.DeepEqual(got, f.Types) {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(f.Types)
				t.Errorf("types Go %s\nTS %s", a, b)
			}
			errors := []failure{}
			for id, e := range result.Errors.All() {
				errors = append(errors, failure{id, e})
			}
			if !reflect.DeepEqual(errors, f.Errors) {
				a, _ := json.Marshal(errors)
				b, _ := json.Marshal(f.Errors)
				t.Errorf("errors Go %s\nTS %s", a, b)
			}
		})
	}
}
