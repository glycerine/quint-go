package quint

import (
	"os"
	"regexp"
	"testing"
)

func TestNameScopeShadowAndRestore(t *testing.T) {
	ids := NewIDGenerator()
	s := NewNameScope("M")
	outer := LookupDefinition{ID: ids.Next(), Name: "x"}
	inner := LookupDefinition{ID: ids.Next(), Name: "x", Depth: 1}
	s.Add(outer)
	added := s.Add(inner)
	if !added.Shadowing || len(s.Errors) != 0 {
		t.Fatal("legal shadow")
	}
	if got, ok := s.Lookup("x"); !ok || got.ID != inner.ID {
		t.Fatal("lookup")
	}
	if got, ok := s.TopLevel("x"); !ok || got.ID != outer.ID {
		t.Fatal("top level")
	}
	s.Pop("x")
	if got, _ := s.Lookup("x"); got.ID != outer.ID {
		t.Fatal("restore")
	}
	s.Pop("x")
	s.Pop("x")
	if _, ok := s.Lookup("x"); ok {
		t.Fatal("empty stack")
	}
}
func TestNameScopeConflicts(t *testing.T) {
	ids := NewIDGenerator()
	s := NewNameScope("M")
	a := LookupDefinition{ID: ids.Next(), Name: "x"}
	b := LookupDefinition{ID: ids.Next(), Name: "x"}
	s.Add(a)
	s.Add(a)
	if len(s.Errors) != 0 || len(s.Definitions.Get("x")) != 1 {
		t.Fatal("same identity duplicated")
	}
	s.Add(b)
	if len(s.Errors) != 2 || *s.Errors[0].Reference != a.ID || *s.Errors[1].Reference != b.ID {
		t.Fatalf("conflicts %+v", s.Errors)
	}
	if s.Errors[0].Message != "Conflicting definitions found for name 'x' in module 'M'" {
		t.Fatal("conflict wording")
	}
	s.Add(LookupDefinition{ID: ids.Next(), Name: "_"})
	if _, ok := s.Lookup("_"); ok {
		t.Fatal("hole collected")
	}
	s.Add(LookupDefinition{ID: ids.Next(), Name: "map"})
	if len(s.Errors) != 3 || s.Errors[2].Code != QNT101 {
		t.Fatal("builtin conflict")
	}
	if _, ok := s.Lookup("map"); ok {
		t.Fatal("builtin collected")
	}
}
func TestResolutionBuiltinInventory(t *testing.T) {
	data, err := os.ReadFile("../quint/src/names/base.ts")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)export const builtinNames = \[(.*?)\]`).FindSubmatch(data)
	if len(block) != 2 {
		t.Fatal("upstream builtin list missing")
	}
	names := regexp.MustCompile(`'([^']+)'`).FindAllSubmatch(block[1], -1)
	if len(names) != len(builtinResolutionNames) {
		t.Fatal("builtin inventory size")
	}
	for _, name := range names {
		if !builtinResolutionNames[string(name[1])] {
			t.Fatalf("missing %s", name[1])
		}
	}
}
