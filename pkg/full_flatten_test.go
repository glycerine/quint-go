package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestUpstreamFullFlattening(t *testing.T) {
	var fixtures []struct {
		Name          string
		Input, Output []Module
		NextID        string
	}
	data, err := os.ReadFile("testdata/parser/full_flatten.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(f.Input)
			if err != nil || len(linked.Errors) > 0 {
				t.Fatal(err, linked.Errors)
			}
			ids := NewIDGenerator()
			n, err := strconv.Atoi(f.NextID)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < n; i++ {
				ids.Next()
			}
			before, _ := json.Marshal(f.Input)
			got, err := FlattenModules(f.Input, linked.Table, ids, nil, AnalysisOutput{})
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(got.Modules)
			b, _ := json.Marshal(f.Output)
			var av, bv any
			json.Unmarshal(a, &av)
			json.Unmarshal(b, &bv)
			if !reflect.DeepEqual(av, bv) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
			after, _ := json.Marshal(f.Input)
			if string(before) != string(after) {
				t.Fatal("input modules changed")
			}
			relinked, err := ResolveLinkedModules(got.Modules)
			if err != nil || len(relinked.Errors) > 0 {
				t.Fatal(err, relinked.Errors)
			}
		})
	}
}

func TestFullFlatteningAnalysisAndSources(t *testing.T) {
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module A { const c:int val a=c } module B { import A(c=1) as I val b=I::a }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(modules)
	analyzed, err := AnalyzeModules(modules, linked.Table)
	if err != nil || len(analyzed.Diagnostics) > 0 {
		t.Fatal(err, analyzed.Diagnostics)
	}
	original := modules[0].Declarations[1]
	sources := SourceMap{original.ID: Location{}}
	oldTypes := analyzed.Types.Len()
	result, err := FlattenModules(analyzed.Modules, linked.Table, ids, sources, AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Modules) != 3 {
		t.Fatal("instance module absent")
	}
	var copied Declaration
	for _, d := range result.Modules[1].Declarations {
		if d.Name == "B::I::a" {
			copied = d
		}
	}
	if copied.Name == "" || copied.ID == original.ID {
		t.Fatal("instance definition not refreshed")
	}
	if _, ok := sources[copied.ID]; !ok {
		t.Fatal("source metadata absent")
	}
	if _, ok := result.Analysis.Types.Get2(copied.ID.String()); !ok {
		t.Fatal("inferred type absent")
	}
	if _, ok := result.Analysis.Effects.Get2(copied.ID.String()); !ok {
		t.Fatal("inferred effect absent")
	}
	if _, ok := result.Analysis.Modes.Get2(copied.ID.String()); !ok {
		t.Fatal("inferred mode absent")
	}
	if analyzed.Types.Len() != oldTypes {
		t.Fatal("original type table extended")
	}
}

func TestFullFlatteningRejectsUnresolvedNames(t *testing.T) {
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module M { val a=missing }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(modules)
	if _, err := FlattenModules(modules, linked.Table, ids, nil, AnalysisOutput{}); err == nil {
		t.Fatal("unresolved project accepted")
	}
}
