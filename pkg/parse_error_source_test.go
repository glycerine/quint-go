package quint

import (
	"errors"
	"testing"
)

func TestSourceAwareSyntaxErrorFiles(t *testing.T) {
	calls := []func() error{
		func() error { _, _, err := ParseExpressionWithSource("expr.qnt", "1 +"); return err },
		func() error { _, _, err := ParseModulesWithSource("module.qnt", "module M {"); return err },
		func() error { _, _, err := ParseTypeWithSource("type.qnt", "List["); return err },
	}
	for i, want := range []string{"expr.qnt", "module.qnt", "type.qnt"} {
		var syntax *SyntaxErrors
		err := calls[i]()
		if !errors.As(err, &syntax) || syntax.Source != want {
			t.Fatalf("%s: %+v", want, err)
		}
	}
	_, err := ParseExpression("1 +")
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) || syntax.Source != "" {
		t.Fatal("ordinary entry point acquired a file name")
	}
}
func TestMapTypeDiagnosticSourceReference(t *testing.T) {
	text := `module A { var x: Map[int,str] }`
	r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": `module Main { import A from "a" }`, "a.qnt": text}}
	_, sources, err := LoadModulesWithSource(r, r.LookupPath(".", "main.qnt"))
	var d Diagnostic
	if !errors.As(err, &d) || d.Code != QNT015 || d.Reference == nil {
		t.Fatalf("diagnostic %v", err)
	}
	loc := sources[*d.Reference]
	if loc.Source != "a.qnt" || sourceExcerpt(t, text, sources, *d.Reference) != "Map[int,str]" {
		t.Fatalf("map source %+v", loc)
	}
	resolved := ResolveDiagnostic(sources, d)
	if len(resolved.Locations) != 1 || resolved.Locations[0].Source != "a.qnt" {
		t.Fatal("unresolved map diagnostic")
	}
	_, err = ParseType("Map[int,str]")
	if !errors.As(err, &d) || d.Reference != nil {
		t.Fatal("ID-free signature parser allocated a reference")
	}
}
