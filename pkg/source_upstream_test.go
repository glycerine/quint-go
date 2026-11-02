package quint

import (
	"errors"
	"testing"
)

func TestUpstreamLocationsFailureAndUnicode(t *testing.T) {
	result, err := ParseModulesWithLocations("bad", "module M { type lower = int }")
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Reference == nil {
		t.Fatalf("diagnostic %v", err)
	}
	if result.UpstreamSources[*diagnostic.Reference].Source != "bad" || result.Sources[*diagnostic.Reference].Source != "bad" {
		t.Fatal("diagnostic maps unavailable")
	}
	_, err = ParseModulesWithLocations("syntax", "module M {")
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) || syntax.Source != "syntax" {
		t.Fatal("syntax source")
	}
	if _, err = ParseModulesWithLocations("bad", string([]byte{0xff})); err == nil {
		t.Fatal("invalid UTF-8")
	}
	result, err = ParseModulesWithLocations("unicode", `module M { val x = "😀" }`)
	if err != nil {
		t.Fatal(err)
	}
	id := result.Modules[0].Declarations[0].Expr.ID
	raw, editor := result.UpstreamSources[id], result.Sources[id]
	if raw.End.Index-raw.Start.Index != 2 || editor.End.Index-editor.Start.Index != 4 {
		t.Fatal("coordinate systems conflated")
	}
}
