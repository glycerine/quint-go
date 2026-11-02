package quint

import "testing"

func TestEffectSubstitutionChains(t *testing.T) {
	input, err := ParseEffect("(e) => Read[r] & Update[w]")
	if err != nil {
		t.Fatal(err)
	}
	concrete := Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}}}
	subs := []EffectSubstitution{
		{Kind: "effect", Name: "e", Effect: Effect{Kind: "variable", Name: "f"}},
		{Kind: "effect", Name: "f", Effect: Effect{Kind: "concrete"}},
		{Kind: "entity", Name: "r", Entity: Entity{Kind: "variable", Name: "s"}},
		{Kind: "entity", Name: "s", Entity: concrete},
		{Kind: "entity", Name: "w", Entity: Entity{Kind: "concrete"}},
	}
	out, err := ApplyEffectSubstitution(subs, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Params[0].Components) != 0 || len(out.Result.Components) != 1 || out.Result.Components[0].Entity.StateVariables[0].Name != "x" {
		t.Fatal(out)
	}
	if input.Params[0].Name != "e" || len(input.Result.Components) != 2 {
		t.Fatal("input mutated")
	}
	single := ApplyEntitySubstitution(subs, Entity{Kind: "variable", Name: "r"})
	if single.Name != "s" {
		t.Fatal("entity pass should be single step")
	}
}
func TestEffectSubstitutionCyclesAndKinds(t *testing.T) {
	for _, kind := range []string{"effect", "entity"} {
		subs := []EffectSubstitution{{Kind: kind, Name: "a", Effect: Effect{Kind: "variable", Name: "b"}, Entity: Entity{Kind: "variable", Name: "b"}}, {Kind: kind, Name: "b", Effect: Effect{Kind: "variable", Name: "a"}, Entity: Entity{Kind: "variable", Name: "a"}}}
		e := Effect{Kind: "variable", Name: "a"}
		if kind == "entity" {
			e = Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "read", Entity: Entity{Kind: "variable", Name: "a"}}}}
		}
		if _, err := ApplyEffectSubstitution(subs, e); err == nil {
			t.Fatal("cycle accepted")
		}
	}
	e := Effect{Kind: "variable", Name: "a"}
	out, err := ApplyEffectSubstitution([]EffectSubstitution{{Kind: "entity", Name: "a"}, {Kind: "effect", Name: "a", Effect: Effect{Kind: "concrete"}}}, e)
	if err != nil || out.Name != "a" {
		t.Fatal("first-match kind semantics")
	}
}

func TestEffectSubstitutionComposition(t *testing.T) {
	first := []EffectSubstitution{
		{Kind: "effect", Name: "a", Effect: Effect{Kind: "variable", Name: "b"}},
		{Kind: "effect", Name: "b", Effect: Effect{Kind: "concrete"}},
		{Kind: "entity", Name: "r", Entity: Entity{Kind: "variable", Name: "s"}},
		{Kind: "entity", Name: "s", Entity: Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}}}},
	}
	second := []EffectSubstitution{
		{Kind: "effect", Name: "target", Effect: Effect{Kind: "variable", Name: "a"}},
		{Kind: "entity", Name: "entityTarget", Entity: Entity{Kind: "variable", Name: "r"}},
		{Kind: "effect", Name: "a", Effect: Effect{Kind: "concrete"}},
	}
	out, err := ComposeEffectSubstitutions(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 7 || out[0].Effect.Kind != "concrete" || out[1].Entity.Name != "s" || out[2].Name != "a" || out[3].Name != "a" {
		t.Fatal(out)
	}
	out[6].Entity.StateVariables[0].Name = "changed"
	if first[3].Entity.StateVariables[0].Name != "x" {
		t.Fatal("input aliased")
	}
	cycle := []EffectSubstitution{{Kind: "effect", Name: "a", Effect: Effect{Kind: "variable", Name: "b"}}, {Kind: "effect", Name: "b", Effect: Effect{Kind: "variable", Name: "a"}}}
	if _, err = ComposeEffectSubstitutions(cycle, second[:1]); err == nil {
		t.Fatal("cycle")
	}
	if _, err = ComposeEffectSubstitutions(nil, []EffectSubstitution{{Kind: "invalid"}}); err == nil {
		t.Fatal("invalid kind")
	}
}
