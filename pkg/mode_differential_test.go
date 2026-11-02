package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"os"
	"testing"
)

func TestUpstreamModeChecking(t *testing.T) {
	type entry struct {
		ID     string
		Scheme EffectScheme
	}
	type failure struct {
		ID    string
		Error Diagnostic
	}
	type suggestion struct {
		ID        string
		Qualifier Qualifier
	}
	var fixtures []struct {
		Name        string
		Input       []Module
		Effects     []entry
		Errors      []failure
		Suggestions []suggestion
	}
	data, err := os.ReadFile("testdata/parser/modes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			effects := insdict.NewDict[string, EffectScheme]()
			for _, e := range f.Effects {
				effects.Set(e.ID, e.Scheme)
			}
			c := NewModeChecker()
			if err := c.CheckModes(f.Input[0].Declarations, effects); err != nil {
				t.Fatal(err)
			}
			errors := []failure{}
			for id, e := range c.Errors.All() {
				errors = append(errors, failure{id, e})
			}
			suggestions := []suggestion{}
			for id, q := range c.Suggestions.All() {
				suggestions = append(suggestions, suggestion{id, q})
			}
			a, _ := json.Marshal(errors)
			b, _ := json.Marshal(f.Errors)
			if string(a) != string(b) {
				t.Errorf("Go %s\nTS %s", a, b)
			}
			a, _ = json.Marshal(suggestions)
			b, _ = json.Marshal(f.Suggestions)
			if string(a) != string(b) {
				t.Errorf("Go %s\nTS %s", a, b)
			}
		})
	}
}
