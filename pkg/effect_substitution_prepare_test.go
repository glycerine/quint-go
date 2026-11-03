package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
)

func effectPreparationID(number string) ID { id, _ := ParseID(number); return id }
func effectPreparationConcrete(entity Entity, kind string) Effect {
	return Effect{Kind: "concrete", Components: []EffectComponent{{Kind: kind, Entity: entity}}}
}
func effectPreparationArrow(parameter, result Effect) Effect {
	return Effect{Kind: "arrow", Params: []Effect{parameter}, Result: &result}
}

// Compare occurrence sharing as well as visible values. Bare source occurrences
// acquire fresh tokens in each independent call, so anchor existing tokens and
// number freshly allocated tokens in deterministic traversal order.
func effectPreparationGraph(output, input Effect, subs []EffectSubstitution) []int {
	labels := map[*int]int{}
	graph := []int{}
	var entity func(Entity, bool)
	var effect func(Effect, bool)
	entity = func(value Entity, record bool) {
		for _, variable := range value.StateVariables {
			label := 0
			if variable.identity != nil {
				var exists bool
				label, exists = labels[variable.identity]
				if !exists {
					label = len(labels) + 1
					labels[variable.identity] = label
				}
			}
			if record {
				graph = append(graph, label)
			}
		}
		for _, child := range value.Entities {
			entity(child, record)
		}
	}
	effect = func(value Effect, record bool) {
		for _, component := range value.Components {
			entity(component.Entity, record)
		}
		for _, parameter := range value.Params {
			effect(parameter, record)
		}
		if value.Result != nil {
			effect(*value.Result, record)
		}
	}
	for _, substitution := range subs {
		entity(substitution.Entity, false)
		effect(substitution.Effect, false)
	}
	effect(input, false)
	effect(output, true)
	return graph
}

func TestEffectSubstitutionPreparationGuard(t *testing.T) {
	t.Parallel()
	bare := Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}}}
	tracked := Entity{Kind: "concrete", StateVariables: []StateVariable{NewStateVariable("x", ID{})}}
	reference := effectPreparationID("9007199254740993123456789")
	for _, tc := range []struct {
		name string
		subs []EffectSubstitution
		want bool
	}{
		{"nil", nil, false},
		{"empty", []EffectSubstitution{}, false},
		{"identified", []EffectSubstitution{{Kind: "entity", Entity: tracked}}, false},
		{"bare", []EffectSubstitution{{Kind: "entity", Entity: bare}}, true},
		{"nested_union", []EffectSubstitution{{Kind: "entity", Entity: Entity{Kind: "union", Entities: []Entity{tracked, {Kind: "union", Entities: []Entity{bare}}}}}}, true},
		{"concrete_effect", []EffectSubstitution{{Kind: "effect", Effect: effectPreparationConcrete(bare, "read")}}, true},
		{"arrow_parameter", []EffectSubstitution{{Kind: "effect", Effect: effectPreparationArrow(effectPreparationConcrete(bare, "read"), Effect{Kind: "concrete"})}}, true},
		{"arrow_result", []EffectSubstitution{{Kind: "effect", Effect: effectPreparationArrow(Effect{Kind: "concrete"}, effectPreparationArrow(Effect{Kind: "concrete"}, effectPreparationConcrete(bare, "read")))}}, true},
		{"identified_arrow", []EffectSubstitution{{Kind: "effect", Effect: effectPreparationArrow(effectPreparationConcrete(tracked, "read"), effectPreparationConcrete(tracked, "update"))}}, false},
		{"reference", []EffectSubstitution{{Kind: "entity", Entity: Entity{Kind: "variable", Name: "r", Reference: &reference}}}, false},
		{"entity_extra_fields", []EffectSubstitution{{Kind: "entity", Entity: Entity{Kind: "variable", Name: "r", StateVariables: bare.StateVariables}}}, true},
		{"effect_extra_fields", []EffectSubstitution{{Kind: "effect", Effect: Effect{Kind: "variable", Name: "e", Components: []EffectComponent{{Entity: bare}}}}}, true},
		{"unknown_binding_ignored", []EffectSubstitution{{Kind: "unknown", Entity: bare, Effect: effectPreparationConcrete(bare, "read")}}, false},
		{"unused_effect_value", []EffectSubstitution{{Kind: "entity", Entity: tracked, Effect: effectPreparationConcrete(bare, "read")}}, false},
		{"unused_entity_value", []EffectSubstitution{{Kind: "effect", Entity: bare, Effect: effectPreparationConcrete(tracked, "read")}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := effectSubstitutionsNeedPreparation(tc.subs); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
}

func TestEffectSubstitutionPreparationOracle(t *testing.T) {
	t.Parallel()
	reference := effectPreparationID("9007199254740993123456789")
	variable := Entity{Kind: "variable", Name: "r", Reference: &reference}
	tracked := Entity{Kind: "concrete", StateVariables: []StateVariable{NewStateVariable("x", reference)}}
	other := Entity{Kind: "concrete", StateVariables: []StateVariable{NewStateVariable("x", reference)}}
	bare := Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x", Reference: reference}}}
	entities := []Entity{
		{Kind: "concrete"}, {Kind: "concrete", StateVariables: []StateVariable{}},
		tracked, bare, {Kind: "variable", Name: "unbound", Reference: &reference},
		{Kind: "union", Entities: []Entity{tracked, tracked, other}},
		{Kind: "union", Entities: []Entity{tracked, bare, {Kind: "variable", Name: "unbound"}}},
		{Kind: "union", Entities: []Entity{variable, tracked}},
	}
	for index, replacement := range entities {
		for _, kind := range []string{"read", "update", "temporal"} {
			for _, arrow := range []bool{false, true} {
				t.Run(fmt.Sprintf("entity%d/%s/arrow=%v", index, kind, arrow), func(t *testing.T) {
					t.Parallel()
					input := effectPreparationConcrete(variable, kind)
					if arrow {
						input = effectPreparationArrow(input, input)
					}
					subs := []EffectSubstitution{{Kind: "entity", Name: "r", Entity: replacement}}
					assertEffectPreparationOracle(t, subs, input)
				})
			}
		}
	}
	for _, tc := range []struct {
		name  string
		subs  []EffectSubstitution
		input Effect
	}{
		{"no_substitutions", nil, effectPreparationConcrete(bare, "read")},
		{"empty_substitutions", []EffectSubstitution{}, Effect{Kind: "concrete", Components: []EffectComponent{}}},
		{"entity_chain", []EffectSubstitution{{Kind: "entity", Name: "r", Entity: Entity{Kind: "variable", Name: "s"}}, {Kind: "entity", Name: "s", Entity: tracked}}, effectPreparationConcrete(variable, "read")},
		{"effect_chain", []EffectSubstitution{{Kind: "effect", Name: "e", Effect: Effect{Kind: "variable", Name: "f"}}, {Kind: "effect", Name: "f", Effect: effectPreparationArrow(effectPreparationConcrete(tracked, "read"), effectPreparationConcrete(tracked, "update"))}}, Effect{Kind: "variable", Name: "e"}},
		{"effect_bare", []EffectSubstitution{{Kind: "effect", Name: "e", Effect: effectPreparationArrow(effectPreparationConcrete(bare, "read"), effectPreparationConcrete(bare, "temporal"))}}, Effect{Kind: "variable", Name: "e"}},
		{"kind_shadow", []EffectSubstitution{{Kind: "entity", Name: "e", Entity: tracked}, {Kind: "effect", Name: "e", Effect: Effect{Kind: "concrete"}}}, Effect{Kind: "variable", Name: "e"}},
		{"unknown_shadow", []EffectSubstitution{{Kind: "unknown", Name: "e"}, {Kind: "effect", Name: "e", Effect: Effect{Kind: "concrete"}}}, Effect{Kind: "variable", Name: "e"}},
		{"duplicate_names", []EffectSubstitution{{Kind: "entity", Name: "r", Entity: tracked}, {Kind: "entity", Name: "r", Entity: bare}}, effectPreparationConcrete(variable, "read")},
		{"entity_cycle", []EffectSubstitution{{Kind: "entity", Name: "r", Entity: Entity{Kind: "variable", Name: "s"}}, {Kind: "entity", Name: "s", Entity: variable}}, effectPreparationConcrete(variable, "read")},
		{"effect_cycle", []EffectSubstitution{{Kind: "effect", Name: "e", Effect: Effect{Kind: "variable", Name: "f"}}, {Kind: "effect", Name: "f", Effect: Effect{Kind: "variable", Name: "e"}}}, Effect{Kind: "variable", Name: "e"}},
		{"arrow_cycle", []EffectSubstitution{{Kind: "effect", Name: "e", Effect: effectPreparationArrow(Effect{Kind: "variable", Name: "e"}, Effect{Kind: "concrete"})}}, Effect{Kind: "variable", Name: "e"}},
	} {
		t.Run(tc.name, func(t *testing.T) { t.Parallel(); assertEffectPreparationOracle(t, tc.subs, tc.input) })
	}
}

func assertEffectPreparationOracle(t *testing.T, subs []EffectSubstitution, input Effect) {
	t.Helper()
	got, err := ApplyEffectSubstitution(subs, input)
	want, wantErr := effectSubstitutionPreparedOracle(subs, input)
	if (err == nil) != (wantErr == nil) || fmt.Sprint(err) != fmt.Sprint(wantErr) || !reflect.DeepEqual(got, want) {
		t.Fatalf("preparation changed fixed-point output/error:\ngot %+v %v\nwant %+v %v", got, err, want, wantErr)
	}
	if !slices.Equal(effectPreparationGraph(got, input, subs), effectPreparationGraph(want, input, subs)) {
		t.Fatal("preparation changed distinct/shared occurrence identity")
	}
}

func TestEffectSubstitutionPreparationOwnershipAndConcurrency(t *testing.T) {
	t.Parallel()
	for _, identified := range []bool{false, true} {
		t.Run(fmt.Sprint(identified), func(t *testing.T) {
			t.Parallel()
			reference := effectPreparationID("9007199254740993123456789")
			state := StateVariable{Name: "x", Reference: reference}
			if identified {
				state = NewStateVariable("x", reference)
			}
			entity := Entity{Kind: "union", Entities: []Entity{{Kind: "variable", Name: "unbound", Reference: &reference}, {Kind: "concrete", StateVariables: []StateVariable{state}}}}
			subs := []EffectSubstitution{{Kind: "entity", Name: "r", Entity: entity}}
			input := effectPreparationArrow(effectPreparationConcrete(Entity{Kind: "variable", Name: "r"}, "read"), effectPreparationConcrete(Entity{Kind: "variable", Name: "r"}, "update"))
			beforeSubs, _ := json.Marshal(subs)
			beforeInput, _ := json.Marshal(input)
			baseline, err := ApplyEffectSubstitution(subs, input)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(baseline)
			var workers sync.WaitGroup
			for range 8 {
				workers.Go(func() {
					for range 4 {
						result, err := ApplyEffectSubstitution(subs, input)
						encoded, encodeErr := json.Marshal(result)
						if err != nil || encodeErr != nil || !bytes.Equal(encoded, want) {
							t.Error("concurrent substitution changed output", err, encodeErr)
							return
						}
						poisonEffectPreparationResult(&result)
					}
				})
			}
			workers.Wait()
			afterSubs, _ := json.Marshal(subs)
			afterInput, _ := json.Marshal(input)
			afterBaseline, _ := json.Marshal(baseline)
			if !bytes.Equal(beforeSubs, afterSubs) || !bytes.Equal(beforeInput, afterInput) || !bytes.Equal(want, afterBaseline) || (subs[0].Entity.Entities[1].StateVariables[0].identity != nil) != identified {
				t.Fatal("substitution exposed source/replacement/result storage or initialized caller identities")
			}
		})
	}
}

func poisonEffectPreparationResult(effect *Effect) {
	var entity func(*Entity)
	entity = func(value *Entity) {
		if value.Reference != nil {
			*value.Reference = ID{}
		}
		for i := range value.StateVariables {
			value.StateVariables[i].Name = "changed"
			value.StateVariables[i].Reference = ID{}
		}
		for i := range value.Entities {
			entity(&value.Entities[i])
		}
	}
	for i := range effect.Components {
		entity(&effect.Components[i].Entity)
	}
	for i := range effect.Params {
		poisonEffectPreparationResult(&effect.Params[i])
	}
	if effect.Result != nil {
		poisonEffectPreparationResult(effect.Result)
	}
}
