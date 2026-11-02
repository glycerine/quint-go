package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamEffectSubstitution(t *testing.T) {
	var fixtures []struct {
		Name          string
		Input, Output Effect
		Substitutions []struct {
			Kind, Name string
			Value      json.RawMessage
		}
	}
	data, err := os.ReadFile("testdata/parser/effect_substitution.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			subs := []EffectSubstitution{}
			for _, s := range f.Substitutions {
				v := EffectSubstitution{Kind: s.Kind, Name: s.Name}
				if s.Kind == "effect" {
					err = json.Unmarshal(s.Value, &v.Effect)
				} else {
					err = json.Unmarshal(s.Value, &v.Entity)
				}
				if err != nil {
					t.Fatal(err)
				}
				subs = append(subs, v)
			}
			out, err := ApplyEffectSubstitution(subs, f.Input)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(out)
			b, _ := json.Marshal(f.Output)
			if string(a) != string(b) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
}
