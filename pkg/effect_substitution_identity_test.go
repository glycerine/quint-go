package quint

import (
	"encoding/json"
	"testing"
)

// The saved oracle below comes from effects/substitutions.ts applySubstitution,
// called directly on JSON-decoded inputs with json-bigint. Each singleton union
// collapses to the bound variable during upstream flattenUnions/simplify.
func TestUpstreamEffectIdentityUnionSubstitutions(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name  string
		Input Effect
		Subs  []struct {
			Kind, Name string
			Value      Entity
		}
		Output Effect
	}
	if err := json.Unmarshal([]byte(effectIdentityUnionOracle), &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			subs := make([]EffectSubstitution, len(fixture.Subs))
			for i, sub := range fixture.Subs {
				subs[i] = EffectSubstitution{Kind: sub.Kind, Name: sub.Name, Entity: sub.Value}
			}
			before, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ApplyEffectSubstitution(subs, fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(fixture.Output)
			if string(a) != string(b) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
			again, err := ApplyEffectSubstitution(subs, got)
			c, _ := json.Marshal(again)
			if err != nil || string(c) != string(b) {
				t.Fatal("identity substitution did not stabilize", string(c), err)
			}
			if got.Kind == "concrete" && got.Components[0].Entity.Reference != nil {
				changed, _ := ParseID("99")
				*got.Components[0].Entity.Reference = changed
			}
			after, _ := json.Marshal(fixture)
			if string(before) != string(after) {
				t.Fatal("substitution mutated input")
			}
		})
	}
}

func TestEffectIdentityUnionDoesNotMaskExpandingCycles(t *testing.T) {
	t.Parallel()
	v := Entity{Kind: "variable", Name: "v"}
	input := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "update", Entity: v}}}
	for _, subs := range [][]EffectSubstitution{
		{{Kind: "entity", Name: "v", Entity: Entity{Kind: "union", Entities: []Entity{v, v}}}},
		{{Kind: "entity", Name: "v", Entity: Entity{Kind: "union", Entities: []Entity{v, {Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}}}}}}},
		{{Kind: "entity", Name: "v", Entity: Entity{Kind: "variable", Name: "w"}}, {Kind: "entity", Name: "w", Entity: v}},
	} {
		if _, err := ApplyEffectSubstitution(subs, input); err == nil {
			t.Fatal("expanding or oscillating update substitution accepted")
		}
	}
}

const effectIdentityUnionOracle = `[{"name":"read singleton union","input":{"kind":"concrete","components":[{"kind":"read","entity":{"kind":"variable","name":"v"}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"variable","name":"v"}]}}],"output":{"kind":"concrete","components":[{"kind":"read","entity":{"kind":"variable","name":"v"}}]}},{"name":"update singleton union","input":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"variable","name":"v"}]}}],"output":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]}},{"name":"temporal singleton union","input":{"kind":"concrete","components":[{"kind":"temporal","entity":{"kind":"variable","name":"v"}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"variable","name":"v"}]}}],"output":{"kind":"concrete","components":[{"kind":"temporal","entity":{"kind":"variable","name":"v"}}]}},{"name":"nested singleton union with empty concrete members","input":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"concrete","stateVariables":[]},{"kind":"union","entities":[{"kind":"union","entities":[{"kind":"variable","name":"v"}]},{"kind":"concrete","stateVariables":[]}]},{"kind":"concrete","stateVariables":[]}]}}],"output":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]}},{"name":"replacement reference preserved","input":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v","reference":7}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"variable","name":"v","reference":9}]}}],"output":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v","reference":9}}]}},{"name":"indirect wrapper chain","input":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"variable","name":"w"}]}},{"kind":"entity","name":"w","value":{"kind":"union","entities":[{"kind":"variable","name":"w"}]}}],"output":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"w"}}]}},{"name":"union mixed with concrete state","input":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"union","entities":[{"kind":"variable","name":"v"},{"kind":"concrete","stateVariables":[{"name":"x","reference":4}]}]}}]},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"concrete","stateVariables":[]},{"kind":"union","entities":[{"kind":"union","entities":[{"kind":"variable","name":"v"}]},{"kind":"concrete","stateVariables":[]}]},{"kind":"concrete","stateVariables":[]}]}}],"output":{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"union","entities":[{"kind":"variable","name":"v"},{"kind":"concrete","stateVariables":[{"name":"x","reference":4}]}]}}]}},{"name":"arrow parameter and result","input":{"kind":"arrow","params":[{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]}],"result":{"kind":"concrete","components":[{"kind":"temporal","entity":{"kind":"variable","name":"v"}}]}},"subs":[{"kind":"entity","name":"v","value":{"kind":"union","entities":[{"kind":"concrete","stateVariables":[]},{"kind":"union","entities":[{"kind":"union","entities":[{"kind":"variable","name":"v"}]},{"kind":"concrete","stateVariables":[]}]},{"kind":"concrete","stateVariables":[]}]}}],"output":{"kind":"arrow","params":[{"kind":"concrete","components":[{"kind":"update","entity":{"kind":"variable","name":"v"}}]}],"result":{"kind":"concrete","components":[{"kind":"temporal","entity":{"kind":"variable","name":"v"}}]}}}]`
