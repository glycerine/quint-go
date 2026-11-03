package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamDependencyCycleOrder(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Direct []struct {
			Name  string
			Input []struct {
				ID   ID
				Uses []ID
			}
			Declarations, Sorted []Declaration
			Cycles               []ID
		}
		Projects []struct {
			Name, Source  string
			Input, Output []Module
			Diagnostics   []Diagnostic
		}
	}
	data, err := os.ReadFile("testdata/parser/dependency_cycle_order.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Direct) != 14 || len(fixtures.Projects) != 6 {
		t.Fatal("dependency cycle inventory", len(fixtures.Direct), len(fixtures.Projects))
	}
	for _, fixture := range fixtures.Direct {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			graph := insdict.NewDict[string, []ID]()
			for _, edge := range fixture.Input {
				graph.Set(edge.ID.String(), edge.Uses)
			}
			type graphEntry struct {
				Key  string
				Uses []ID
			}
			snapshot := func() []graphEntry {
				entries := []graphEntry{}
				for key, uses := range graph.All() {
					entries = append(entries, graphEntry{key, append([]ID{}, uses...)})
				}
				return entries
			}
			before := snapshot()
			sorted, cycles := SortDeclarations(graph, fixture.Declarations)
			if !reflect.DeepEqual(sorted, fixture.Sorted) || !reflect.DeepEqual(cycles, fixture.Cycles) {
				t.Fatalf("ordered topological result\n got %+v %+v\nwant %+v %+v", sorted, cycles, fixture.Sorted, fixture.Cycles)
			}
			if !reflect.DeepEqual(before, snapshot()) {
				t.Fatal("mutated dependency graph")
			}
		})
	}
	for _, fixture := range fixtures.Projects {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			modules, err := ParseModules(fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(modules, fixture.Input) {
				got, _ := json.Marshal(modules)
				want, _ := json.Marshal(fixture.Input)
				t.Fatalf("raw parsed input differs\n got %s\nwant %s", got, want)
			}
			resolved, err := ResolveLinkedModules(modules)
			if err != nil || len(resolved.Errors) > 0 {
				t.Fatal(err, resolved.Errors)
			}
			before, _ := json.Marshal(modules)
			got, diagnostics := SortModuleDeclarations(modules, resolved.Table)
			if !reflect.DeepEqual(got, fixture.Output) || !reflect.DeepEqual(diagnostics, fixture.Diagnostics) {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(diagnostics)
				t.Fatalf("source cycle phase\n modules %s\n diagnostics %s\nwant %+v", a, b, fixture.Diagnostics)
			}
			after, _ := json.Marshal(modules)
			if string(before) != string(after) {
				t.Fatal("mutated input modules")
			}
		})
	}
}
