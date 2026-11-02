package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamCombinedAnalysis(t *testing.T) {
	type entry struct {
		ID     string
		Scheme EffectScheme
	}
	type mode struct {
		ID        string
		Qualifier Qualifier
	}
	var fixtures []struct {
		Name    string
		Input   []Module
		Errors  []Diagnostic
		Effects []entry
		Modes   []mode
	}
	data, err := os.ReadFile("testdata/parser/analyzer.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			got, err := AnalyzeModules(f.Input, linked.Table)
			if err != nil {
				t.Fatal(err)
			}
			effects := []entry{}
			for id, s := range got.Effects.All() {
				effects = append(effects, entry{id, s})
			}
			modes := []mode{}
			for id, q := range got.Modes.All() {
				modes = append(modes, mode{id, q})
			}
			for _, pair := range [][2]any{{got.Diagnostics, f.Errors}, {effects, f.Effects}, {modes, f.Modes}} {
				a, _ := json.Marshal(pair[0])
				b, _ := json.Marshal(pair[1])
				if string(a) != string(b) {
					t.Errorf("Go %s\nTS %s", a, b)
				}
			}
		})
	}
}
