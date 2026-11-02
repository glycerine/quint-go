package quint

import (
	"encoding/json"
	"testing"
)

func TestEffectSchemeInstantiation(t *testing.T) {
	e, err := ParseEffect("(a, Read[r, free]) => b")
	if err != nil {
		t.Fatal(err)
	}
	s := EffectScheme{Effect: e, EffectVariables: []string{"b", "a"}, EntityVariables: []string{"r"}}
	var fresh FreshNames
	for _, want := range []string{"(_e1, Read[_v0, free]) => _e0", "(_e3, Read[_v1, free]) => _e2"} {
		got, err := InstantiateEffectScheme(s, &fresh)
		if err != nil || EffectString(got) != want {
			t.Fatalf("%s %v", EffectString(got), err)
		}
		got.Result.Name = "changed"
	}
	if e.Result.Name != "b" {
		t.Fatal("input mutated")
	}
	// First-match kind behavior differs intentionally between inference composition
	// (entity first) and diagnostic concatenation (effect first), as upstream.
	both, _ := ParseEffect("(a) => Read[a]")
	s = EffectScheme{Effect: both, EffectVariables: []string{"a"}, EntityVariables: []string{"a"}}
	got, err := InstantiateEffectScheme(s, &FreshNames{})
	if err != nil || EffectString(got) != "(a) => Read[_v0]" {
		t.Fatalf("%+v %v", got, err)
	}
	text, err := EffectSchemeString(s)
	if err != nil || text != "∀ e0, v0 . (e0) => Read[a]" {
		t.Fatalf("%s %v", text, err)
	}
}

func TestEffectSchemeFormatting(t *testing.T) {
	for _, tc := range []struct {
		input  string
		es, vs []string
		want   string
	}{
		{"Pure", nil, nil, "Pure"},
		{"(a, Read[r, free]) => b", []string{"b", "a"}, []string{"r"}, "∀ e0, e1, v0 . (e1, Read[v0, free]) => e0"},
		{"Read[r] & Update[w]", nil, []string{"w", "r"}, "∀ v0, v1 . Read[v1] & Update[v0]"},
	} {
		e, err := ParseEffect(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := EffectSchemeString(EffectScheme{Effect: e, EffectVariables: tc.es, EntityVariables: tc.vs})
		if err != nil || got != tc.want {
			t.Fatalf("%s %v", got, err)
		}
	}
}

func TestEffectNamespaces(t *testing.T) {
	input, err := ParseEffect("(e, Read[r, 'x']) => Update['y'] & Temporal['z']")
	if err != nil {
		t.Fatal(err)
	}
	input.Result.Components[0].Entity.StateVariables[0].Name = "existing::y"
	before, _ := json.Marshal(input)
	for _, tc := range []struct {
		ns   []string
		want string
	}{
		{nil, "(e, Read[r, 'x']) => Update['existing::y'] & Temporal['z']"},
		{[]string{"Inner", "Outer"}, "(e, Read[r, 'Outer::Inner::x']) => Update['Outer::Inner::existing::y'] & Temporal['Outer::Inner::z']"},
	} {
		got := AddEffectNamespaces(input, tc.ns)
		if EffectString(got) != tc.want {
			t.Fatal(EffectString(got))
		}
		if got.Result.Components[0].Entity.StateVariables[0].Reference != input.Result.Components[0].Entity.StateVariables[0].Reference {
			t.Fatal("reference changed")
		}
		got.Result.Components[0].Entity.StateVariables[0].Name = "changed"
	}
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("mutated input")
	}
}
