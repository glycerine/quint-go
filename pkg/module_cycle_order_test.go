package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamModuleCycleOrdering(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name         string
		InputModules []Module
		Modules      []Module
		Diagnostics  []Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/module_cycle_order.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 15 {
		t.Fatalf("module ordering fixture inventory: got %d, want 15", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			before, err := json.Marshal(fixture.InputModules)
			if err != nil {
				t.Fatal(err)
			}
			modules, diagnostics := SortModules(fixture.InputModules)
			if !reflect.DeepEqual(modules, fixture.Modules) {
				got, _ := json.Marshal(modules)
				want, _ := json.Marshal(fixture.Modules)
				t.Errorf("ordered modules:\n got %s\nwant %s", got, want)
			}
			if !reflect.DeepEqual(diagnostics, fixture.Diagnostics) {
				got, _ := json.Marshal(diagnostics)
				want, _ := json.Marshal(fixture.Diagnostics)
				t.Errorf("ordered diagnostics:\n got %s\nwant %s", got, want)
			}
			after, err := json.Marshal(fixture.InputModules)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("module sorting mutated input:\n got %s\nwant %s", after, before)
			}
		})
	}
}
