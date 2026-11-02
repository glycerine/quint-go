package quint

import "testing"

func TestMatchConstraintSolving(t *testing.T) {
	id, _ := ParseID("1")
	result := Type{Kind: "var", Name: "r"}
	sum := Type{Kind: "sum", Fields: &Row{Kind: "row", Fields: []RowField{{Name: "Ok", Type: Type{Kind: "int"}}, {Name: "Err", Type: Type{Kind: "str"}}}, Other: &Row{Kind: "empty"}}}
	elim := func(label, payload, ret string) []TypedExpression {
		output := Type{Kind: ret}
		return []TypedExpression{{Expr: Expr{Kind: "str", Str: label}, Type: Type{Kind: "str"}}, {Expr: Expr{Kind: "name", Name: "handler"}, Type: Type{Kind: "oper", Args: []Type{{Kind: payload}}, Res: &output}}}
	}
	for _, tc := range []struct {
		label, ret string
		success    bool
	}{{"Err", "bool", true}, {"_", "bool", true}, {"Err", "int", false}, {"Missing", "bool", false}} {
		args := []TypedExpression{{Expr: Expr{Kind: "name", Name: "value"}, Type: sum}}
		args = append(args, elim("Ok", "int", "bool")...)
		args = append(args, elim(tc.label, "str", tc.ret)...)
		cs, es := MatchConstraints(id, args, result)
		if len(es) != 0 {
			t.Fatal(es)
		}
		solved, err := (TypeUnifier{}).SolveConstraint(Constraint{Kind: "conjunction", Constraints: cs})
		if err != nil {
			t.Fatal(err)
		}
		failures := 0
		for range solved.Errors.All() {
			failures++
		}
		if (failures == 0) != tc.success {
			t.Fatalf("%+v %+v", tc, solved)
		}
		if tc.success {
			out, err := ApplySubstitution(solved.Substitutions, result)
			if err != nil || out.Kind != "bool" {
				t.Fatal(out, err)
			}
		}
	}
}
func TestMatchConstraintValidation(t *testing.T) {
	id, _ := ParseID("1")
	r := Type{Kind: "var", Name: "r"}
	ret := Type{Kind: "int"}
	label := func(s string) TypedExpression { return TypedExpression{Expr: Expr{Kind: "str", Str: s}} }
	unary := TypedExpression{Expr: Expr{Kind: "name", Name: "f"}, Type: Type{Kind: "oper", Args: []Type{{Kind: "int"}}, Res: &ret}}
	receiver := TypedExpression{Expr: Expr{Kind: "name", Name: "x"}, Type: Type{Kind: "var", Name: "s"}}
	for _, labels := range [][]string{{"_", "Ok"}, {"_", "_"}} {
		cs, es := MatchConstraints(id, []TypedExpression{receiver, label(labels[0]), unary, label(labels[1]), unary}, r)
		if cs != nil || len(es) != 1 || es[0].Message == nil {
			t.Fatal(cs, es)
		}
	}
	bad := TypedExpression{Expr: Expr{Kind: "name", Name: "notLabel"}, Type: Type{Kind: "int"}}
	binary := unary
	binary.Type.Args = []Type{{Kind: "int"}, {Kind: "int"}}
	_, es := MatchConstraints(id, []TypedExpression{receiver, bad, unary, label("A"), bad, label("B"), binary}, r)
	if len(es) != 3 {
		t.Fatal(es)
	}
	if *es[0].Message != "Match variant name must be a string literal but it is a name: notLabel" {
		t.Fatal(es[0])
	}
	if _, es = MatchConstraints(id, nil, r); len(es) != 1 {
		t.Fatal("arity")
	}
}
