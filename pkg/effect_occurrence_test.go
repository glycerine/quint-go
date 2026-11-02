package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestUpstreamEffectOccurrenceBoundaries(t *testing.T) {
	t.Parallel()
	type substitution struct {
		Kind, Name string
		Value      json.RawMessage
	}
	var fixtures struct {
		Substitutions []struct {
			Name          string
			Input, Output Effect
			Substitutions []substitution
		}
		Entities []struct {
			Name          string
			Input, Output Entity
			Substitutions []substitution
		}
		Compositions []struct {
			Name                  string
			First, Second, Output []substitution
		}
		Unification []struct {
			Name          string
			A, B          Entity
			Substitutions []substitution
			Error         *ErrorTree
		}
		Occurrences []struct {
			Name, Origin, Kind string
			Seed, Output       Effect
		}
	}
	data, err := os.ReadFile("testdata/parser/effect_occurrence.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	decode := func(t *testing.T, input []substitution) []EffectSubstitution {
		t.Helper()
		out := make([]EffectSubstitution, len(input))
		for i, s := range input {
			out[i] = EffectSubstitution{Kind: s.Kind, Name: s.Name}
			var err error
			if s.Kind == "effect" {
				err = json.Unmarshal(s.Value, &out[i].Effect)
			} else {
				err = json.Unmarshal(s.Value, &out[i].Entity)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	jsonEqual := func(t *testing.T, got, want any) {
		t.Helper()
		a, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("Go %s\nTypeScript %s", a, b)
		}
	}
	for _, f := range fixtures.Substitutions {
		t.Run(f.Name, func(t *testing.T) {
			subs := decode(t, f.Substitutions)
			before, _ := json.Marshal(f.Input)
			out, err := ApplyEffectSubstitution(subs, f.Input)
			if err != nil {
				t.Fatal(err)
			}
			jsonEqual(t, out, f.Output)
			again, err := ApplyEffectSubstitution(subs, out)
			if err != nil {
				t.Fatal(err)
			}
			jsonEqual(t, again, out)
			after, _ := json.Marshal(f.Input)
			if string(before) != string(after) {
				t.Fatal("substitution mutated its input")
			}
		})
	}
	for _, f := range fixtures.Entities {
		t.Run("entity/"+f.Name, func(t *testing.T) {
			jsonEqual(t, ApplyEntitySubstitution(decode(t, f.Substitutions), f.Input), f.Output)
		})
	}
	for _, f := range fixtures.Compositions {
		t.Run("compose/"+f.Name, func(t *testing.T) {
			first, second := decode(t, f.First), decode(t, f.Second)
			out, err := ComposeEffectSubstitutions(first, second)
			if err != nil {
				t.Fatal(err)
			}
			want := decode(t, f.Output)
			if len(out) != len(want) {
				t.Fatalf("composition Go %+v\nTypeScript %+v", out, want)
			}
			for i, s := range out {
				if s.Kind != want[i].Kind || s.Name != want[i].Name {
					t.Fatalf("binding %d Go %+v\nTypeScript %+v", i, s, want[i])
				}
				if s.Kind == "effect" {
					jsonEqual(t, s.Effect, want[i].Effect)
				} else {
					jsonEqual(t, s.Entity, want[i].Entity)
				}
			}
		})
	}
	for _, f := range fixtures.Unification {
		t.Run("unify/"+f.Name, func(t *testing.T) {
			before, _ := json.Marshal([2]Entity{f.A, f.B})
			out, failure := UnifyEntities(f.A, f.B)
			if !reflect.DeepEqual(failure, f.Error) {
				t.Fatalf("error Go %+v\nTypeScript %+v", failure, f.Error)
			}
			want := decode(t, f.Substitutions)
			if len(out) != len(want) {
				t.Fatalf("substitutions Go %+v\nTypeScript %+v", out, want)
			}
			for i, s := range out {
				if s.Kind != want[i].Kind || s.Name != want[i].Name {
					t.Fatalf("binding %d Go %+v\nTypeScript %+v", i, s, want[i])
				}
				jsonEqual(t, s.Entity, want[i].Entity)
			}
			after, _ := json.Marshal([2]Entity{f.A, f.B})
			if string(before) != string(after) {
				t.Fatal("unification mutated its inputs")
			}
		})
	}
	for _, f := range fixtures.Occurrences {
		t.Run("occurrence/"+f.Name, func(t *testing.T) {
			shared := f.Seed.Components[0].Entity
			var distinct Effect
			if f.Origin == "parser" {
				var err error
				f.Seed, err = ParseEffect("Read['x']")
				if err != nil {
					t.Fatal(err)
				}
				shared = f.Seed.Components[0].Entity
				distinct, err = ParseEffect("Read['x']")
				if err != nil {
					t.Fatal(err)
				}
			} else if f.Origin == "state-json" || f.Origin == "native" {
				state := f.Seed.Components[0].Entity.StateVariables[0]
				other := state
				if f.Origin == "state-json" {
					data, _ := json.Marshal(state)
					if err := json.Unmarshal(data, &state); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(data, &other); err != nil {
						t.Fatal(err)
					}
				} else {
					state = NewStateVariable(state.Name, state.Reference)
					other = NewStateVariable(other.Name, other.Reference)
				}
				shared = Entity{Kind: "concrete", StateVariables: []StateVariable{state}}
				distinct = Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "read", Entity: Entity{Kind: "concrete", StateVariables: []StateVariable{other}}}}}
			} else {
				data, _ := json.Marshal(f.Seed)
				if err := json.Unmarshal(data, &distinct); err != nil {
					t.Fatal(err)
				}
			}
			input := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: f.Kind, Entity: Entity{Kind: "union", Entities: []Entity{shared, shared, distinct.Components[0].Entity}}}}}
			before, _ := json.Marshal(input)
			out := SimplifyEffect(input)
			jsonEqual(t, out, f.Output)
			out.Components[0].Entity.StateVariables[0].Name = "changed"
			after, _ := json.Marshal(input)
			if string(before) != string(after) {
				t.Fatal("simplification mutated the shared occurrence")
			}
		})
	}
}

func TestStateVariableJSONPreservesValuesAndFailedReceiver(t *testing.T) {
	t.Parallel()
	ref, err := ParseID("1234567890123456789012345678901234567890")
	if err != nil {
		t.Fatal(err)
	}
	state := NewStateVariable("x", ref)
	original := state
	if err := json.Unmarshal([]byte(`{"name":"changed","reference":-1}`), &state); err == nil {
		t.Fatal("invalid reference accepted")
	}
	if state.Name != original.Name || state.Reference != original.Reference || state.identity != original.identity {
		t.Fatal("failed decoding changed its receiver")
	}
	if err := json.Unmarshal([]byte(" \nnull\t"), &state); err != nil {
		t.Fatal(err)
	}
	if state.Name != original.Name || state.Reference != original.Reference || state.identity != original.identity {
		t.Fatal("null decoding changed its receiver")
	}
	if err := json.Unmarshal([]byte(`{"name":"y"}`), &state); err != nil {
		t.Fatal(err)
	}
	if state.Name != "y" || state.Reference != ref || state.identity == original.identity {
		t.Fatal("partial decoding lost the reference or retained an old occurrence")
	}
}

// The upstream implementation recursively reapplies these non-stabilizing
// substitutions until the JS stack is exhausted. Go reports them promptly.
func TestEffectSubstitutionNonterminatingCycles(t *testing.T) {
	t.Parallel()
	v := func(name string) Entity { return Entity{Kind: "variable", Name: name} }
	u := func(members ...Entity) Entity { return Entity{Kind: "union", Entities: members} }
	ref, err := ParseID("1")
	if err != nil {
		t.Fatal(err)
	}
	state := Entity{Kind: "concrete", StateVariables: []StateVariable{NewStateVariable("x", ref)}}
	patterns := []struct {
		name string
		subs []EffectSubstitution
	}{
		{"repeated-self", []EffectSubstitution{{Kind: "entity", Name: "v", Entity: u(v("v"), v("v"))}}},
		{"state-emitting-self", []EffectSubstitution{{Kind: "entity", Name: "v", Entity: u(v("v"), state)}}},
		{"unbound-emitting-self", []EffectSubstitution{{Kind: "entity", Name: "v", Entity: u(v("v"), v("w"))}}},
		{"branching-mutual", []EffectSubstitution{{Kind: "entity", Name: "v", Entity: u(v("v"), v("w"))}, {Kind: "entity", Name: "w", Entity: v("v")}}},
		{"cycle-emitting-self", []EffectSubstitution{{Kind: "entity", Name: "v", Entity: u(v("v"), v("w"))}, {Kind: "entity", Name: "w", Entity: v("w")}}},
		{"state-emitting-mutual", []EffectSubstitution{{Kind: "entity", Name: "v", Entity: v("w")}, {Kind: "entity", Name: "w", Entity: u(v("v"), state)}}},
	}
	for _, f := range patterns {
		t.Run(f.name, func(t *testing.T) {
			input := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "update", Entity: v("v")}}}
			if _, err := ApplyEffectSubstitution(f.subs, input); err == nil {
				t.Fatal("non-terminating update substitution accepted")
			}
		})
		t.Run("effect-alias-"+f.name, func(t *testing.T) {
			subs := append([]EffectSubstitution{}, f.subs...)
			for i := range 6 {
				value := Effect{Kind: "variable", Name: "e" + strconv.Itoa(i+1)}
				if i == 5 {
					value = Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "update", Entity: v("v")}}}
				}
				subs = append(subs, EffectSubstitution{Kind: "effect", Name: "e" + strconv.Itoa(i), Effect: value})
			}
			if _, err := ApplyEffectSubstitution(subs, Effect{Kind: "variable", Name: "e0"}); err == nil {
				t.Fatal("effect aliases masked a non-terminating update substitution")
			}
		})
	}
	for _, kind := range []string{"read", "temporal", "update"} {
		t.Run("oscillating-"+kind, func(t *testing.T) {
			subs := []EffectSubstitution{{Kind: "entity", Name: "v", Entity: v("w")}, {Kind: "entity", Name: "w", Entity: v("v")}}
			input := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: kind, Entity: v("v")}}}
			if _, err := ApplyEffectSubstitution(subs, input); err == nil {
				t.Fatal("oscillating entity substitution accepted")
			}
		})
	}
	for _, indirect := range []bool{false, true} {
		variable := Effect{Kind: "variable", Name: "e"}
		result := variable
		if indirect {
			result.Name = "f"
		}
		subs := []EffectSubstitution{{Kind: "effect", Name: "e", Effect: Effect{Kind: "arrow", Params: []Effect{}, Result: &result}}}
		if indirect {
			subs = append(subs, EffectSubstitution{Kind: "effect", Name: "f", Effect: variable})
		}
		if _, err := ApplyEffectSubstitution(subs, variable); err == nil {
			t.Fatal("expanding arrow substitution accepted")
		}
	}
}
