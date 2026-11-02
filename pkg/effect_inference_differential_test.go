package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamEffectInference(t *testing.T) {
	type entry struct {
		ID     string
		Scheme EffectScheme
	}
	type failure struct {
		ID    string
		Error ErrorTree
	}
	var fixtures []struct {
		Name    string
		Input   []Module
		Effects []entry
		Errors  []failure
	}
	data, err := os.ReadFile("testdata/parser/effect_inference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			g := NewEffectInferrer(linked.Table)
			g.InferEffects(f.Input[0].Declarations)
			got := []entry{}
			for id, s := range g.Effects.All() {
				got = append(got, entry{id, s})
			}
			errors := []failure{}
			for id, e := range g.Errors.All() {
				errors = append(errors, failure{id, e})
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(f.Effects)
			if string(a) != string(b) {
				t.Errorf("Go %s\nTS %s", a, b)
			}
			a, _ = json.Marshal(errors)
			b, _ = json.Marshal(f.Errors)
			if string(a) != string(b) {
				t.Errorf("Go errors %s\nTS errors %s", a, b)
			}
		})
	}
}
