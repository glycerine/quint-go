package quint

import "testing"

func TestParseExpressionPrecedence(t *testing.T) {
	e, err := ParseExpression("1 + 2 * 3 ^ 4 ^ 5")
	if err != nil {
		t.Fatal(err)
	}
	if e.Opcode != "iadd" || e.Args[1].Opcode != "imul" || e.Args[1].Args[1].Opcode != "ipow" || e.Args[1].Args[1].Args[1].Opcode != "ipow" {
		t.Fatalf("precedence: %+v", e)
	}
	seen := map[ID]bool{}
	if err = WalkExpr(&e, func(e *Expr) (bool, error) {
		if seen[e.ID] {
			t.Fatal("duplicate ID")
		}
		seen[e.ID] = true
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestParseExpressionForms(t *testing.T) {
	for _, c := range [][2]string{{"1 - 2", "isub"}, {"4 / 2", "idiv"}, {"4 % 2", "imod"}, {"-1", "iuminus"}, {"1 == 2", "eq"}, {"1 != 2", "neq"}, {"1 < 2", "ilt"}, {"1 <= 2", "ilte"}, {"1 > 2", "igt"}, {"1 >= 2", "igte"}, {"true and false", "and"}, {"true or false", "or"}, {"and { true, false }", "and"}, {"or { true, false }", "or"}, {"all { true, false }", "actionAll"}, {"any { true, false }", "actionAny"}, {"(1, 2)", "Tup"}, {"1 -> 2", "Tup"}, {"()", "Tup"}, {"[1, 2]", "List"}, {"[]", "List"}, {"if (true) 1 else 2", "ite"}} {
		e, err := ParseExpression(c[0])
		if err != nil || e.Opcode != c[1] {
			t.Fatalf("%s: %+v %v", c[0], e, err)
		}
	}
	for _, s := range []string{"x", "true", "false", "\"hello\"", "123456789012345678901234567890", "(1)", "{1}"} {
		if _, err := ParseExpression(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"", "1 +", "1 = 2", "1 trailing"} {
		if _, err := ParseExpression(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestCallAndAccessConversion(t *testing.T) {
	for _, c := range []struct {
		source, opcode string
		arity          int
	}{{"f()", "f", 0}, {"f(1, g(2))", "f", 2}, {"xs.append(1)", "append", 2}, {"xs.head()", "head", 1}, {"xs[0]", "nth", 2}, {"r.foo", "field", 2}, {"t._1", "item", 2}, {"t._99", "item", 2}, {"t._100", "field", 2}, {"t._0", "field", 2}, {"x' = x + 1", "assign", 2}} {
		e, err := ParseExpression(c.source)
		if err != nil || e.Opcode != c.opcode || len(e.Args) != c.arity {
			t.Fatalf("%s: %+v %v", c.source, e, err)
		}
	}
	e, err := ParseExpression("r.foo")
	if err != nil || e.Args[1].Kind != "str" || e.Args[1].Str != "foo" {
		t.Fatal("field encoding", err)
	}
	e, err = ParseExpression("t._2")
	if err != nil || e.Args[1].Int.String() != "2" {
		t.Fatal("tuple encoding", err)
	}
	e, err = ParseExpression("x' = 1")
	if err != nil || e.Args[0].Name != "x" || e.Args[1].Int.String() != "1" {
		t.Fatal("assignment encoding", err)
	}
	e, err = ParseExpression(`"a\nb"`)
	if err != nil || e.Str != `a\nb` {
		t.Fatalf("upstream string semantics: %q %v", e.Str, err)
	}
	for _, s := range []string{"f(", "x.", "x' ="} {
		if _, err := ParseExpression(s); err == nil {
			t.Errorf("expected syntax or unsupported conversion error: %s", s)
		}
	}
}

func TestLambdaAndRecordConversion(t *testing.T) {
	e, err := ParseExpression("(x, _) => x + 1")
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != "lambda" || e.Qualifier != Def || len(e.Params) != 2 || e.Params[0].Name != "x" || e.Params[1].Name != "_" || e.Body.Opcode != "iadd" {
		t.Fatalf("lambda %+v", e)
	}
	e, err = ParseExpression("xs.map(x => x + 1)")
	if err != nil || e.Args[1].Kind != "lambda" {
		t.Fatalf("lambda argument: %+v %v", e, err)
	}
	e, err = ParseExpression("{ a: 1, b: true }")
	if err != nil {
		t.Fatal(err)
	}
	if e.Opcode != "Rec" || len(e.Args) != 4 || e.Args[0].Str != "a" || e.Args[2].Str != "b" {
		t.Fatalf("record %+v", e)
	}
	e, err = ParseExpression("{ a: 1, ...r, b: 2 }")
	if err != nil {
		t.Fatal(err)
	}
	if e.Opcode != "with" || e.Args[1].Str != "b" || e.Args[0].Opcode != "with" || e.Args[0].Args[0].Name != "r" {
		t.Fatalf("spread %+v", e)
	}
	e, err = ParseExpression("{ ...r }")
	if err != nil || e.Kind != "name" || e.Name != "r" {
		t.Fatalf("spread only %+v %v", e, err)
	}
	for _, c := range [][2]string{{"{a: 1, a: 2}", QNT016}, {"{...a, ...b}", QNT012}} {
		_, err := ParseExpression(c[0])
		d, ok := err.(Diagnostic)
		if !ok || d.Code != c[1] {
			t.Fatalf("%s: %v", c[0], err)
		}
	}
}
