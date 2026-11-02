package quint

import (
	"errors"
	"testing"
)

func TestDeprecatedMapType(t *testing.T) {
	for _, tc := range []struct{ source, original, replacement string }{
		{"Map[int,str]", "Map[int, str]", "int -> str"},
		{"List[Map[Set[int], List[str]]]", "Map[Set[int], List[str]]", "Set[int] -> List[str]"},
		{"Map[a,b]", "Map[a, b]", "a -> b"},
	} {
		_, err := ParseType(tc.source)
		var d Diagnostic
		if !errors.As(err, &d) || d.Code != QNT015 || d.Data == nil || d.Data.Fix == nil {
			t.Fatalf("%s: %v", tc.source, err)
		}
		fix := d.Data.Fix
		if fix.Kind != "replace" || fix.Original != tc.original || fix.Replacement != tc.replacement {
			t.Fatalf("fix %+v", fix)
		}
		want := "Use '" + tc.replacement + "' instead of '" + tc.original + "' for map types"
		if d.Message != want {
			t.Fatalf("message %q", d.Message)
		}
		typ, err := ParseType(fix.Replacement)
		if err != nil || typ.Kind != "fun" {
			t.Fatalf("replacement %+v %v", typ, err)
		}
	}
	for _, source := range []string{"Map[int]", "Map[int,str,bool]", "ns::Map[int,str]"} {
		typ, err := ParseType(source)
		if err != nil || typ.Kind != "app" {
			t.Fatalf("nondeprecated application %s: %+v %v", source, typ, err)
		}
	}
}

func TestMapDiagnosticAcrossEntryPoints(t *testing.T) {
	calls := []func() error{
		func() error { _, err := ParseDeclaration("var x: Map[int,str]"); return err },
		func() error { _, err := ParseModules("module M { const x: Map[int,str] }"); return err },
		func() error { _, err := ParseOperatorDefinition("def f(x: Map[int,str]): bool = true"); return err },
		func() error { _, err := ParseExpressionOrDeclaration("type T = Map[int,str]", nil); return err },
	}
	for _, call := range calls {
		var d Diagnostic
		if err := call(); !errors.As(err, &d) || d.Code != QNT015 {
			t.Fatalf("diagnostic: %v", err)
		}
	}
}
