package quint

import "testing"

func TestDebugCallDesugaring(t *testing.T) {
	for _, tc := range []struct{ source, label, kind string }{
		{`q::debug(1 + 2)`, "q::debug(1+2)", "app"},
		{`q::debug(/* value */ 0xFF)`, "q::debug(0xFF)", "int"},
		{`q::debug("hello world")`, `q::debug("hello world")`, "str"},
		{`x.q::debug()`, "x.q::debug()", "name"},
	} {
		e, err := ParseExpression(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		if e.Opcode != "q::debug" || len(e.Args) != 2 || e.Args[0].Kind != "str" || e.Args[0].Str != tc.label || e.Args[1].Kind != tc.kind {
			t.Fatalf("%s: %+v", tc.source, e)
		}
		if e.ID == e.Args[0].ID || e.Args[0].ID == e.Args[1].ID {
			t.Fatal("duplicate synthesized label ID")
		}
	}
	e, err := ParseExpression("q::debug(q::debug(x))")
	if err != nil {
		t.Fatal(err)
	}
	if e.Args[0].Str != "q::debug(q::debug(x))" || e.Args[1].Args[0].Str != "q::debug(x)" {
		t.Fatal("nested labels")
	}
}

func TestDebugCallExplicitArguments(t *testing.T) {
	for _, tc := range []struct {
		source string
		arity  int
	}{
		{`q::debug()`, 0}, {`q::debug("label", x)`, 2}, {`q::debug(1,2,3)`, 3},
		{`x.q::debug("label")`, 2}, {`debug(x)`, 1}, {`other::debug(x)`, 1},
	} {
		e, err := ParseExpression(tc.source)
		if err != nil || len(e.Args) != tc.arity {
			t.Fatalf("%s: %+v %v", tc.source, e, err)
		}
	}
	ms, err := ParseModules("module M { val x = q::debug(1) }")
	if err != nil {
		t.Fatal(err)
	}
	if ms[0].Declarations[0].Expr.Args[0].Str != "q::debug(1)" {
		t.Fatal("module conversion")
	}
}
