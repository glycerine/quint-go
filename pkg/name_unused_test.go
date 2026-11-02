package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamUnusedDefinitions(t *testing.T) {
	type definition struct{ ID, Name string }
	var fixtures []struct {
		Name   string
		Input  []Module
		Unused [][]definition
	}
	data, err := os.ReadFile("testdata/parser/unused.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			r, err := ResolveLinkedModules(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			for i, m := range f.Input {
				got := []definition{}
				for _, d := range r.UnusedDefinitions(m.Name) {
					got = append(got, definition{d.ID.String(), d.Name})
				}
				if !reflect.DeepEqual(got, f.Unused[i]) {
					t.Fatalf("%s: Go %+v TS %+v", m.Name, got, f.Unused[i])
				}
			}
		})
	}
}

func TestUnusedBindingIdentity(t *testing.T) {
	ms, err := ParseModules("module A { val x=1 val y=x } module B { import A.* val z=1 } module C { import A.* val z=x }")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 0 {
		t.Fatalf("%v %+v", err, r.Errors)
	}
	names := func(module string) []string {
		out := []string{}
		for _, d := range r.UnusedDefinitions(module) {
			out = append(out, d.Name)
		}
		return out
	}
	for module, want := range map[string][]string{"A": {"y"}, "B": {"x", "y", "z"}, "C": {"y", "z"}, "missing": {}} {
		if got := names(module); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v", module, got)
		}
	}
}

func TestFrontendUnusedDefinitions(t *testing.T) {
	resolver := &fixtureSourceResolver{sources: map[string]string{"main.qnt": "module M { val x=1 val y=x }"}}
	r, err := ParseProject(resolver, resolver.LookupPath(".", "main.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	unused := r.UnusedDefinitions("M")
	if len(unused) != 1 || unused[0].Name != "y" {
		t.Fatal(unused)
	}
	if len((FrontendResult{}).UnusedDefinitions("M")) != 0 {
		t.Fatal("empty frontend")
	}
}
