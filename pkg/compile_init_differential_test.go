package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"os"
	"testing"
)

func TestUpstreamInitConversion(t *testing.T) {
	var fixtures []struct {
		Name   string
		Input  Module
		Output *Module
		Errors []Diagnostic
		Modes  []struct {
			ID        string
			Qualifier Qualifier
		}
	}
	data, err := os.ReadFile("testdata/parser/convert_init.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules([]Module{f.Input})
			modes := insdict.NewDict[string, Qualifier]()
			for _, m := range f.Modes {
				modes.Set(m.ID, m.Qualifier)
			}
			out, errors := ConvertInit(f.Input, linked.Table, modes)
			if len(errors) != len(f.Errors) {
				t.Fatalf("Go %+v TS %+v", errors, f.Errors)
			}
			if len(errors) > 0 {
				a, _ := json.Marshal(errors)
				b, _ := json.Marshal(f.Errors)
				if string(a) != string(b) {
					t.Fatalf("Go %s TS %s", a, b)
				}
			} else {
				a, _ := json.Marshal(out)
				b, _ := json.Marshal(f.Output)
				if string(a) != string(b) {
					t.Fatalf("Go %s TS %s", a, b)
				}
			}
		})
	}
}
