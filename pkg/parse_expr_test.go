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
