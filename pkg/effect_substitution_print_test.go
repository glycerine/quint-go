package quint

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestUpstreamEffectSubstitutionsPrinting(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/parser/api_effect_printing.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name   string
		Input  []EffectSubstitution
		Output string
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		before := effectSubstitutionPrintSnapshot(t, fixture.Input)
		got, err := EffectSubstitutionsString(fixture.Input)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s: got %q want %q: %v", fixture.Name, got, fixture.Output, err)
		}
		if after := effectSubstitutionPrintSnapshot(t, fixture.Input); after != before {
			t.Fatalf("%s changed borrowed input", fixture.Name)
		}
	}
	if got, err := EffectSubstitutionsString(nil); err != nil || got != "[]" {
		t.Fatal(got, err)
	}
}

func effectSubstitutionPrintSnapshot(t *testing.T, substitutions []EffectSubstitution) string {
	t.Helper()
	input := make([]struct {
		Kind, Name string
		Value      any
	}, len(substitutions))
	for i, substitution := range substitutions {
		input[i].Kind, input[i].Name = substitution.Kind, substitution.Name
		if substitution.Kind == "effect" {
			input[i].Value = substitution.Effect
		} else {
			input[i].Value = substitution.Entity
		}
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEffectSubstitutionsPrintingValidation(t *testing.T) {
	t.Parallel()
	badEffects := []Effect{
		{}, {Kind: "variable"}, {Kind: "arrow"},
		{Kind: "arrow", Params: []Effect{{Kind: "bad"}}, Result: &Effect{Kind: "concrete"}},
		{Kind: "concrete", Components: []EffectComponent{{Kind: "bad", Entity: Entity{Kind: "concrete"}}}},
		{Kind: "concrete", Components: []EffectComponent{{Kind: "read", Entity: Entity{Kind: "bad"}}}},
	}
	for _, effect := range badEffects {
		if _, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "effect", Effect: effect}}); err == nil {
			t.Fatal("accepted malformed effect", effect)
		}
	}
	for _, entity := range []Entity{{}, {Kind: "variable"}, {Kind: "union", Entities: []Entity{{Kind: "bad"}}}} {
		if _, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "entity", Entity: entity}}); err == nil {
			t.Fatal("accepted malformed entity", entity)
		}
	}
	if _, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "other"}}); err == nil {
		t.Fatal("accepted unknown substitution kind")
	}
	effect := Effect{Kind: "arrow"}
	effect.Result = &effect
	if _, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "effect", Effect: effect}}); err == nil || !strings.Contains(err.Error(), "cyclic effect") {
		t.Fatal("accepted cyclic effect", err)
	}
	entity := Entity{Kind: "union", Entities: make([]Entity, 1)}
	entity.Entities[0] = entity
	if _, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "entity", Entity: entity}}); err == nil || !strings.Contains(err.Error(), "cyclic entity") {
		t.Fatal("accepted cyclic entity", err)
	}
	// A shared subtree is a valid DAG, unlike a cycle on the active path.
	result := Effect{Kind: "concrete"}
	shared := Effect{Kind: "arrow", Result: &result}
	outer := Effect{Kind: "arrow", Params: []Effect{shared, shared}, Result: &result}
	if text, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "effect", Name: "f", Effect: outer}}); err != nil || text != "[f |-> (() => Pure, () => Pure) => Pure]" {
		t.Fatal(text, err)
	}
	sharedEntities := []Entity{{Kind: "variable", Name: "x"}}
	entityDAG := Entity{Kind: "union", Entities: []Entity{{Kind: "union", Entities: sharedEntities}, {Kind: "union", Entities: sharedEntities}}}
	if text, err := EffectSubstitutionsString([]EffectSubstitution{{Kind: "entity", Name: "v", Entity: entityDAG}}); err != nil || text != "[v |-> x, x]" {
		t.Fatal(text, err)
	}
}
