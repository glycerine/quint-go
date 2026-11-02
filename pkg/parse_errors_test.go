package quint

import (
	"errors"
	"strings"
	"testing"
)

func TestSyntaxErrorPositions(t *testing.T) {
	for _, tc := range []struct {
		source   string
		position Position
	}{
		{`"😀" +`, Position{Line: 0, Col: 5, Index: 5}},
		{"(\n", Position{Line: 1, Col: 0, Index: 2}},
		{"(\r\n", Position{Line: 1, Col: 0, Index: 3}},
		{`"😀" trailing`, Position{Line: 0, Col: 5, Index: 5}},
		{"1 +\n@", Position{Line: 1, Col: 0, Index: 4}},
	} {
		_, err := ParseExpression(tc.source)
		var syntax *SyntaxErrors
		if !errors.As(err, &syntax) || len(syntax.Issues) == 0 {
			t.Fatalf("%q: %v", tc.source, err)
		}
		if syntax.Issues[0].Position != tc.position {
			t.Fatalf("%q: %+v want %+v", tc.source, syntax.Issues[0], tc.position)
		}
	}
}
func TestSyntaxCodesAndAccumulation(t *testing.T) {
	_, err := ParseExpression("1 = 2")
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) || syntax.Issues[0].Code != QNT006 {
		t.Fatalf("grammar diagnostic %v", err)
	}
	if !strings.Contains(err.Error(), "invalid expression:") || !strings.Contains(err.Error(), "QNT006") {
		t.Fatal("display message")
	}
	_, err = ParseExpression("@ #")
	if !errors.As(err, &syntax) || len(syntax.Issues) < 2 {
		t.Fatalf("multiple lexer errors %v", err)
	}
}
func TestSyntaxErrorsAcrossEntryPoints(t *testing.T) {
	calls := []func() error{
		func() error { _, err := ParseType("List["); return err },
		func() error { _, err := ParseEffect("Read["); return err },
		func() error { _, err := ParseDeclaration("var x:"); return err },
		func() error { _, err := ParseOperatorDefinition("val x ="); return err },
		func() error { _, err := ParseModules("module M {"); return err },
		func() error { _, err := ParseExpressionOrDeclaration("1 +", nil); return err },
	}
	for _, call := range calls {
		var syntax *SyntaxErrors
		if err := call(); !errors.As(err, &syntax) || len(syntax.Issues) == 0 {
			t.Fatalf("missing structured syntax error %v", err)
		}
	}
}
