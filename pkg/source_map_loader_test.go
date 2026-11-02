package quint

import (
	"errors"
	"reflect"
	"testing"
)

func TestImportSourceMaps(t *testing.T) {
	files := map[string]string{
		"main.qnt":  `module Main { import A from "sub/a" val x = 1 }`,
		"sub/a.qnt": `module A { import B from "../b" val y = 2 }`,
		"b.qnt":     `module B { val z = 3 }`,
	}
	r := &fixtureSourceResolver{sources: files}
	ms, sources, err := LoadModulesWithSource(r, r.LookupPath(".", "main.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := LoadAndSortModules(r, r.LookupPath(".", "main.qnt"))
	if err != nil || !reflect.DeepEqual(ms, plain) {
		t.Fatal("source maps changed module IR/order")
	}
	paths := map[string]string{"Main": "main.qnt", "A": "sub/a.qnt", "B": "b.qnt"}
	for _, m := range ms {
		if sources[m.ID].Source != paths[m.Name] {
			t.Fatalf("module source %+v", sources[m.ID])
		}
		for _, d := range m.Declarations {
			if sources[d.ID].Source != paths[m.Name] {
				t.Fatal("declaration file mismatch")
			}
			if d.Expr != nil && sources[d.Expr.ID].Source != paths[m.Name] {
				t.Fatal("expression file mismatch")
			}
		}
	}
}
func TestImportedDiagnosticLocations(t *testing.T) {
	for _, tc := range []struct{ body, code string }{
		{`module A { import Missing from "missing" }`, QNT013},
		{`module A { import Missing }`, QNT405},
		{`module A { import Main from "main" }`, QNT098},
		{`module A { type lower = int }`, QNT007},
	} {
		r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": `module Main { import A from "a" }`, "a.qnt": tc.body}}
		ms, sources, err := LoadModulesWithSource(r, r.LookupPath(".", "main.qnt"))
		var d Diagnostic
		if ms != nil || !errors.As(err, &d) || d.Code != tc.code {
			t.Fatalf("diagnostic %v", err)
		}
		resolved := ResolveDiagnostic(sources, d)
		if len(resolved.Locations) != 1 {
			t.Fatalf("unresolved diagnostic %+v", resolved)
		}
		if tc.code != QNT098 && resolved.Locations[0].Source != "a.qnt" {
			t.Fatal("wrong error source")
		}
	}
	r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": `module Main { import A from "a" }`, "a.qnt": "module A { val x ="}}
	_, sources, err := LoadModulesWithSource(r, r.LookupPath(".", "main.qnt"))
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) || syntax.Source != "a.qnt" || len(sources) == 0 {
		t.Fatalf("syntax source %v", err)
	}
}
