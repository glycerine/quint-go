package quint

import (
	"reflect"
	"testing"
)

func TestEffectFreeNameDependencies(t *testing.T) {
	e, _ := ParseEffect("(b) => Read[r]")
	before := EffectFreeNames{Effects: []string{"a"}, Entities: []string{"outer"}}
	subs := []EffectSubstitution{
		{Kind: "effect", Name: "unrelated", Effect: Effect{Kind: "variable", Name: "ignored"}},
		{Kind: "effect", Name: "a", Effect: e},
		{Kind: "effect", Name: "b", Effect: Effect{Kind: "variable", Name: "c"}},
		{Kind: "entity", Name: "r", Entity: Entity{Kind: "union", Entities: []Entity{{Kind: "variable", Name: "s"}, {Kind: "variable", Name: "s"}}}},
		{Kind: "entity", Name: "s", Entity: Entity{Kind: "variable", Name: "tail"}},
	}
	got := ExpandEffectFreeNames(before, subs)
	want := EffectFreeNames{Effects: []string{"a", "b", "c"}, Entities: []string{"outer", "r", "s", "tail"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	got.Effects[0] = "changed"
	if before.Effects[0] != "a" {
		t.Fatal("input mutated")
	}
	reversed := []EffectSubstitution{subs[2], subs[1]}
	got = ExpandEffectFreeNames(before, reversed)
	if !reflect.DeepEqual(got.Effects, []string{"a", "b"}) {
		t.Fatal("unexpected transitive closure")
	}
}
func TestGeneralizeEffect(t *testing.T) {
	e, err := ParseEffect("(b, a) => Read[r, s, r] & Update[w]")
	if err != nil {
		t.Fatal(err)
	}
	s := GeneralizeEffect(e, EffectFreeNames{Effects: []string{"a"}, Entities: []string{"s"}})
	if !reflect.DeepEqual(s.EffectVariables, []string{"b"}) || !reflect.DeepEqual(s.EntityVariables, []string{"r", "w"}) {
		t.Fatal(s)
	}
	s.Effect.Params[0].Name = "changed"
	if e.Params[0].Name != "b" {
		t.Fatal("input mutated")
	}
	pure := GeneralizeEffect(Effect{Kind: "concrete"}, EffectFreeNames{})
	if len(pure.EffectVariables) != 0 || len(pure.EntityVariables) != 0 {
		t.Fatal(pure)
	}
}
