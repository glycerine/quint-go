package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestUpstreamCompilation(t *testing.T) {
	var fixtures []struct {
		Name, NextID string
		Input        []Module
		Args         CompilationOptions
		Output       *Module
		Errors       []struct{ Explanation string }
	}
	data, err := os.ReadFile("testdata/parser/compile.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			ids := NewIDGenerator()
			n, err := strconv.Atoi(f.NextID)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < n; i++ {
				ids.Next()
			}
			linked, _ := ResolveLinkedModules(f.Input)
			analyzed, err := AnalyzeModules(f.Input, linked.Table)
			if err != nil || len(analyzed.Diagnostics) > 0 {
				t.Fatal(err, analyzed.Diagnostics)
			}
			before, _ := json.Marshal(f.Input)
			got, err := CompileModules(analyzed.Modules, AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes}, ids, nil, func() CompilationOptions { o := f.Args; o.Main = "M"; return o }())
			if err != nil {
				t.Fatal(err)
			}
			if f.Output == nil {
				if len(got.Diagnostics) != len(f.Errors) {
					t.Fatal(got.Diagnostics, f.Errors)
				}
				for i, d := range got.Diagnostics {
					if !strings.Contains(f.Errors[i].Explanation, d.Message) || !strings.Contains(f.Errors[i].Explanation, d.Code) {
						t.Fatal(d, f.Errors[i])
					}
				}
				return
			}
			if len(got.Diagnostics) > 0 {
				t.Fatal(got.Diagnostics)
			}
			if f.Name == "expression" && (len(got.AnalysisDiagnostics) != 1 || got.AnalysisDiagnostics[0].Code != QNT200) {
				t.Fatal("incremental diagnostics not retained", got.AnalysisDiagnostics)
			}
			a, _ := json.Marshal(got.MainModule)
			b, _ := json.Marshal(f.Output)
			var av, bv any
			json.Unmarshal(a, &av)
			json.Unmarshal(b, &bv)
			if !reflect.DeepEqual(av, bv) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
			after, _ := json.Marshal(f.Input)
			if string(before) != string(after) {
				t.Fatal("input changed")
			}
			for _, d := range got.MainModule.Declarations {
				if strings.HasPrefix(d.Name, "q::") {
					if _, ok := got.Analysis.Types.Get2(d.ID.String()); !ok {
						t.Fatal("wrapper type absent")
					}
					if _, ok := got.Analysis.Effects.Get2(d.ID.String()); !ok {
						t.Fatal("wrapper effect absent")
					}
				}
			}
		})
	}
}

func TestCompilationArgumentFailures(t *testing.T) {
	for _, tc := range []struct{ name, source, input, flag, code, message string }{
		{"variable", "module M { var init:bool action step=true }", "init", "init", QNT501, "used by a var declaration"},
		{"constant", "module M { const init:bool action step=true }", "init", "init", QNT501, "used by a const declaration"},
		{"step", "module M { action init=true val step=true }", "step", "step", QNT501, "default --step"},
		{"declaration", "module M { action step=true }", "val x=1", "init", QNT501, "valid expression"},
		{"syntax", "module M { action step=true }", "(", "init", QNT501, "valid expression"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(tc.source, ids)
			if err != nil {
				t.Fatal(err)
			}
			linked, _ := ResolveLinkedModules(modules)
			analysis, err := AnalyzeModules(modules, linked.Table)
			if err != nil {
				t.Fatal(err)
			}
			options := CompilationOptions{Main: "M", CompilationEntries: CompilationEntries{Target: "json", Flatten: true}}
			if tc.flag == "init" {
				options.Init = &tc.input
			} else {
				options.Step = &tc.input
			}
			sources := SourceMap{}
			got, err := CompileModules(analysis.Modules, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}, ids, sources, options)
			if err != nil || len(got.Diagnostics) != 1 {
				t.Fatal(err, got.Diagnostics)
			}
			if d := got.Diagnostics[0]; d.Code != tc.code || !strings.Contains(d.Message, tc.message) {
				t.Fatal(d)
			}
		})
	}
}

func TestCompilationMissingMain(t *testing.T) {
	modules, _ := ParseModules("module M { val a=1 }")
	if _, err := CompileModules(modules, AnalysisOutput{}, NewIDGenerator(), nil, CompilationOptions{Main: "Missing"}); err == nil || err.Error() != "module Missing does not exist" {
		t.Fatal(err)
	}
}
