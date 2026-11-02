package quint

import (
	"errors"
	"reflect"
	"testing"
)

func TestImportedUpstreamSourceMaps(t *testing.T) {
	r := &fixtureSourceResolver{sources: map[string]string{
		"main.qnt": `module Main { import A from "a" }`,
		"a.qnt":    "module A {\n val s = \"😀\"\n}",
	}}
	result, err := LoadModulesWithLocations(r, r.LookupPath(".", "main.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	modules, sources, err := LoadModulesWithSource(r, r.LookupPath(".", "main.qnt"))
	if err != nil || !reflect.DeepEqual(modules, result.Modules) || !reflect.DeepEqual(sources, result.Sources) {
		t.Fatal("dual-map loading changed existing results")
	}
	if len(result.Sources) != len(result.UpstreamSources) {
		t.Fatal("map coverage differs")
	}
	for id, loc := range result.Sources {
		raw, ok := result.UpstreamSources[id]
		if !ok || raw.Source != loc.Source {
			t.Fatal("source identity differs")
		}
	}
	expr := result.Modules[0].Declarations[0].Expr
	raw := result.UpstreamSources[expr.ID]
	editor := result.Sources[expr.ID]
	if raw.Source != "a.qnt" || raw.End.Index-raw.Start.Index != 2 || editor.End.Index-editor.Start.Index != 4 {
		t.Fatal("imported Unicode span")
	}
}
func TestImportedUpstreamDiagnosticMaps(t *testing.T) {
	for _, body := range []string{`module A { import Missing from "missing" }`, `module A { type lower = int }`, `module A { import Main from "main" }`} {
		r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": `module Main { import A from "a" }`, "a.qnt": body}}
		result, err := LoadModulesWithLocations(r, r.LookupPath(".", "main.qnt"))
		var d Diagnostic
		if !errors.As(err, &d) || d.Reference == nil || result.Modules != nil {
			t.Fatalf("diagnostic %v", err)
		}
		if _, ok := result.UpstreamSources[*d.Reference]; !ok {
			t.Fatal("missing upstream diagnostic reference")
		}
		if _, ok := result.Sources[*d.Reference]; !ok {
			t.Fatal("missing UTF-16 diagnostic reference")
		}
	}
	r := &fixtureSourceResolver{sources: map[string]string{}}
	result, err := LoadModulesWithLocations(r, r.LookupPath(".", "missing.qnt"))
	if err == nil || len(result.Sources) != 0 || len(result.UpstreamSources) != 0 {
		t.Fatal("failed main-file load")
	}
}
