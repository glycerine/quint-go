package quint

import (
	"errors"
	"reflect"
	"testing"
)

func TestProjectFrontend(t *testing.T) {
	r := &fixtureSourceResolver{sources: map[string]string{
		"main.qnt": "module Main { import A(N=base) as I from \"a\" val result=I::x val base=2 }",
		"a.qnt":    "module A { val x=N const N:int }",
	}}
	result, err := ParseProject(r, r.LookupPath(".", "main.qnt"))
	if err != nil || len(result.Diagnostics) != 0 {
		t.Fatalf("%v %+v", err, result.Diagnostics)
	}
	if len(result.Modules) != 2 || result.Modules[0].Name != "A" || result.Modules[1].Name != "Main" {
		t.Fatal("module order")
	}
	names := []string{}
	for _, d := range result.Modules[1].Declarations {
		names = append(names, d.Kind+":"+d.Name)
	}
	if !reflect.DeepEqual(names, []string{"def:base", "instance:", "def:result"}) {
		t.Fatal(names)
	}
	if result.Scopes.Get("I") == nil {
		t.Fatal("missing instance alias")
	}
	x := result.Modules[0].Declarations[1]
	ref := result.Modules[1].Declarations[2].Expr.ID
	if result.Table.Get(ref).ID != x.ID {
		t.Fatal("cross-file reference")
	}
	if result.Sources[ref].Source != "main.qnt" || result.UpstreamSources[ref].Source != "main.qnt" {
		t.Fatal("source maps")
	}
	if result.IDs == nil {
		t.Fatal("missing continuation ID generator")
	}
	next := result.IDs.Next()
	if _, exists := result.Sources[next]; exists {
		t.Fatal("continued generator reused a parsed ID")
	}
}

func TestProjectFrontendAccumulatedDiagnostics(t *testing.T) {
	r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": "module M { val good=1 val missing=unknown val loop=loop val blocked=loop }"}}
	result, err := ParseProject(r, r.LookupPath(".", "main.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{}
	for _, d := range result.Diagnostics {
		codes = append(codes, d.Code)
		if len(ResolveDiagnostic(result.Sources, d).Locations) != 1 {
			t.Fatalf("missing span %+v", d)
		}
	}
	if !reflect.DeepEqual(codes, []string{QNT404, QNT099, QNT099}) {
		t.Fatal(codes)
	}
	if len(result.Modules[0].Declarations) != 2 {
		t.Fatal("sortable declarations lost")
	}
}

func TestProjectFrontendLoadFailures(t *testing.T) {
	cases := []struct {
		source string
		code   string
	}{
		{"module M { import A from \"a\" }", QNT013},
		{"module M { import A }", QNT405},
		{"module M { type lower=int }", QNT007},
	}
	for _, tc := range cases {
		r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": tc.source}}
		result, err := ParseProject(r, r.LookupPath(".", "main.qnt"))
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Code != tc.code {
			t.Fatalf("%v", err)
		}
		if result.Table != nil || result.Modules != nil || len(result.Sources) == 0 {
			t.Fatal("partial failure state")
		}
	}
	r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": "module M { val x="}}
	_, err := ParseProject(r, r.LookupPath(".", "main.qnt"))
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) {
		t.Fatalf("syntax error: %v", err)
	}
}
