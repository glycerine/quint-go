package quint

import (
	"encoding/json"
	"testing"
)

func TestModuleTypeAnalysis(t *testing.T) {
	resolver := &fixtureSourceResolver{sources: map[string]string{
		"main.qnt": "module Main { import A.* from \"a\" val xs=identity(List(1,2)) val truth=identity(true) }",
		"a.qnt":    "module A { type Box[a]=List[a] def identity(x)=x const c:Box[int] }",
	}}
	parsed, err := ParseProject(resolver, resolver.LookupPath(".", "main.qnt"))
	if err != nil || len(parsed.Diagnostics) != 0 {
		t.Fatalf("%v %+v", err, parsed.Diagnostics)
	}
	before, _ := json.Marshal(parsed.Modules)
	result, err := AnalyzeModuleTypes(parsed.Modules, parsed.Table)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range result.TypeErrors.All() {
		t.Fatal(e)
	}
	for _, e := range result.ApplicationErrors.All() {
		t.Fatal(e)
	}
	for _, m := range result.Modules {
		for _, d := range m.Declarations {
			want := ""
			switch d.Name {
			case "xs":
				want = "List[int]"
			case "truth":
				want = "bool"
			case "c":
				want = "List[int]"
			}
			if want != "" && TypeString(result.Types.Get(d.ID.String()).Type) != want {
				t.Fatalf("%s: %+v", d.Name, result.Types.Get(d.ID.String()))
			}
		}
	}
	after, _ := json.Marshal(parsed.Modules)
	if string(before) != string(after) {
		t.Fatal("source modules mutated")
	}
}
func TestModuleTypeAnalysisFailureResults(t *testing.T) {
	ms, err := ParseModules("module M { type Box[a]=List[a] const c:Box[int,str] val x=1+true }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	result, err := AnalyzeModuleTypes(ms, linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	applicationCount, typeCount := 0, 0
	for range result.ApplicationErrors.All() {
		applicationCount++
	}
	for range result.TypeErrors.All() {
		typeCount++
	}
	if applicationCount != 1 || typeCount != 1 {
		t.Fatal(applicationCount, typeCount)
	}
	if len(result.Modules) != 1 || len(result.Modules[0].Declarations) != 3 {
		t.Fatal("partial modules lost")
	}
}
