package quint

import "testing"

func TestCompilationStageViewsAndFailedFlagReferences(t *testing.T) {
	for _, flatten := range []bool{false, true} {
		ids := NewIDGenerator()
		modules, err := parseModulesWithIDs("module M { var x: int action init=x'=0 action step=x'=x+1 }", ids)
		if err != nil {
			t.Fatal(err)
		}
		linked, err := ResolveLinkedModules(modules)
		if err != nil {
			t.Fatal(err)
		}
		analysis, err := AnalyzeModules(modules, linked.Table)
		if err != nil || len(analysis.Diagnostics) != 0 {
			t.Fatal(err, analysis.Diagnostics)
		}
		options := CompilationOptions{Main: "M", CompilationEntries: CompilationEntries{Target: "json", Flatten: flatten}}
		compiled, err := CompileModules(analysis.Modules, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}, ids, nil, options)
		if err != nil || len(compiled.Diagnostics) != 0 || len(compiled.UnflattenedModules) != 1 {
			t.Fatal(err, compiled.Diagnostics, compiled.UnflattenedModules)
		}
		view := compiled.UnflattenedModules[0]
		want := 3
		if flatten {
			want += 2
		}
		if len(view.Declarations) != want || len(modules[0].Declarations) != 3 {
			t.Fatal("source view does not preserve source and synthesized entries", view)
		}
		compiled.UnflattenedModules[0].Name = "changed"
		view.Declarations[0].Name = "changed"
		if compiled.MainModule.Name != "M" || compiled.MainModule.Declarations[0].Name != "x" || modules[0].Declarations[0].Name != "x" {
			t.Fatal("source view shares mutable module or declaration slices")
		}
	}

	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module M { val init=true action step=true }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(modules)
	if err != nil {
		t.Fatal(err)
	}
	original := linked.Table.Len()
	compiled, err := CompileModules(modules, AnalysisOutput{}, ids, nil, CompilationOptions{Main: "M", CompilationEntries: CompilationEntries{Target: "json", Flatten: true}})
	if err != nil || len(compiled.Diagnostics) != 1 || compiled.Table == nil || compiled.Table.Len() != original+1 || linked.Table.Len() != original || compiled.UnflattenedModules != nil {
		t.Fatal("flag failure lost its successful lookup or changed input lookup", err, compiled)
	}
}
