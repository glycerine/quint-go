package quint

import (
	"errors"
	"reflect"
	"testing"
)

func TestDeclarationSourceEntry(t *testing.T) {
	text := "/// docs\ntype T = A | B"
	ds, m, err := ParseDeclarationWithSource("decl", text)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := ParseDeclaration(text)
	if err != nil || !reflect.DeepEqual(ds, plain) {
		t.Fatal("declaration IR changed")
	}
	if sourceExcerpt(t, text, m, ds[0].ID) != "type T = A | B" || sourceExcerpt(t, text, m, ds[2].ID) != "B" {
		t.Fatal("declaration spans")
	}
	_, m, err = ParseDeclarationWithSource("bad", "val x =")
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) || syntax.Source != "bad" || m != nil {
		t.Fatalf("failure %v", err)
	}
}
func TestREPLSourceSession(t *testing.T) {
	ids := NewIDGenerator()
	all := SourceMap{}
	for i, tc := range []struct{ name, text, kind string }{{"one", "val x: int = 1", "declaration"}, {"two", "x+2", "expr"}, {"three", "// nothing\n", "none"}} {
		result, m, err := ParseREPLWithSource(tc.name, tc.text, ids)
		if err != nil || result.Kind != tc.kind {
			t.Fatalf("result %+v %v", result, err)
		}
		for id, loc := range m {
			if _, exists := all[id]; exists {
				t.Fatal("session ID reused")
			}
			if loc.Source != tc.name {
				t.Fatal("source identity")
			}
			all[id] = loc
		}
		if i == 2 && len(m) != 0 {
			t.Fatal("empty input allocated source IDs")
		}
	}
	_, m, err := ParseREPLWithSource("four", "type lower = int", ids)
	var d Diagnostic
	if !errors.As(err, &d) || d.Reference == nil || m[*d.Reference].Source != "four" {
		t.Fatalf("semantic reference %v", err)
	}
	_, _, err = ParseREPLWithSource("five", "1 +", ids)
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) || syntax.Source != "five" {
		t.Fatal("syntax source")
	}
	result, _, err := ParseREPLWithSource("six", "3", nil)
	if err != nil || result.Expr.Int.String() != "3" {
		t.Fatal("nil generator")
	}
}
