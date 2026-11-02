package quint

import (
	"errors"
	"strings"
	"testing"
)

func TestTypeDeclarationDiagnostics(t *testing.T) {
	for _, tc := range []struct{ source, code, variables string }{
		{"type lower", QNT007, ""}, {"type ns::lower = int", QNT007, ""},
		{"type T = List[a]", QNT014, "a"}, {"type T[a] = (a,b,b)", QNT014, "b"},
		{"type T = {x:a | r}", QNT014, "a, r"}, {"type T[a] = A(a) | B(b)", QNT014, "b"},
	} {
		_, err := ParseDeclaration(tc.source)
		var d Diagnostic
		if !errors.As(err, &d) || d.Code != tc.code || d.Reference == nil {
			t.Fatalf("%q: %v", tc.source, err)
		}
		if tc.variables != "" && !strings.HasPrefix(d.Message, "the type variables "+tc.variables+" are unbound.") {
			t.Fatalf("message %q", d.Message)
		}
		if tc.code == QNT007 && (d.Data == nil || d.Data.Fix == nil || d.Data.Fix.Kind != "replace") {
			t.Fatal("missing replacement")
		}
		if _, err := ParseModules("module M { " + tc.source + " }"); err == nil {
			t.Fatal("module skipped check")
		}
		if _, err := ParseExpressionOrDeclaration(tc.source, nil); err == nil {
			t.Fatal("REPL skipped check")
		}
	}
	_, err := ParseDeclaration("type ns::lower = int")
	var d Diagnostic
	errors.As(err, &d)
	if d.Data.Fix.Original != "type ns::lower" || d.Data.Fix.Replacement != "type ns::Lower" {
		t.Fatalf("fix %+v", d.Data.Fix)
	}
}

func TestDeclaredTypeVariables(t *testing.T) {
	for _, source := range []string{"type T[a,r] = {x:a | r}", "type T[a] = A(a) | B", "type lower = A | B", "type _internal", "type ns::Upper = int"} {
		if _, err := ParseDeclaration(source); err != nil {
			t.Fatalf("%q: %v", source, err)
		}
	}
}
