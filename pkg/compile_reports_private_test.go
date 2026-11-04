package quint

import "testing"

func TestCompilationCustomReportNamesAndCommaPredicates(t *testing.T) {
	t.Parallel()
	names := []string{"q::privateReport"}
	options := CompilationEntries{Target: "json", Invariants: []string{"true,false"}, InvariantReports: true, InvariantReportNames: names}
	declarations, err := SynthesizeCompilationEntries(options, NewIDGenerator())
	if err != nil || len(declarations) != 2 || declarations[1].Name != names[0] || declarations[1].Expr.Opcode != "and" || len(declarations[1].Expr.Args) != 2 {
		t.Fatal(declarations, err)
	}
	if options.Invariants[0] != "true,false" || names[0] != "q::privateReport" {
		t.Fatal("synthesis changed caller options")
	}
	parsed, err := ParseModulesWithLocations("M.qnt", "module M { action init=true action step=true }")
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(parsed.Modules)
	if err != nil || len(linked.Errors) > 0 {
		t.Fatal(err, linked.Errors)
	}
	analysis, err := AnalyzeModules(parsed.Modules, linked.Table)
	if err != nil || len(analysis.Diagnostics) > 0 {
		t.Fatal(err, analysis.Diagnostics)
	}
	compiled, err := CompileModules(analysis.Modules, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}, parsed.IDs, parsed.Sources, CompilationOptions{Main: "M", CompilationEntries: options})
	if err != nil || len(compiled.Diagnostics) > 0 || len(compiled.AnalysisDiagnostics) > 0 {
		t.Fatal(err, compiled.Diagnostics, compiled.AnalysisDiagnostics)
	}
	names[0] = "caller changed"
	if compiled.InvariantReportNames[0] != "q::privateReport" {
		t.Fatal("compiled report names alias caller options", compiled.InvariantReportNames)
	}
}
