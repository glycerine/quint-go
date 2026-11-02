package quint

import "testing"

func TestRecordSpecialConstraints(t *testing.T) {
	id, _ := ParseID("1")
	result := Type{Kind: "var", Name: "r"}
	rec, _ := ParseType("{x:int,y:str}")
	typed := func(e Expr, t Type) TypedExpression { return TypedExpression{Expr: e, Type: t} }
	receiver := typed(Expr{Kind: "name", Name: "record"}, rec)
	key := typed(Expr{Kind: "str", Str: "x"}, Type{Kind: "str"})
	one, _ := ParseInteger("1")
	value := typed(Expr{Kind: "int", Int: one}, Type{Kind: "int"})
	for _, tc := range []struct {
		op   string
		args []TypedExpression
		want string
	}{
		{"Rec", []TypedExpression{key, value}, "{ x: int }"},
		{"Rec", nil, "{}"},
		{"field", []TypedExpression{receiver, key}, "int"},
		{"fieldNames", []TypedExpression{receiver}, "Set[str]"},
		{"with", []TypedExpression{receiver, key, value}, "{ x: int, y: str }"},
	} {
		constraints, e := RecordConstraints(tc.op, id, tc.args, result)
		if e != nil {
			t.Fatal(e)
		}
		solution, err := (TypeUnifier{}).SolveConstraint(Constraint{Kind: "conjunction", Constraints: constraints})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range solution.Errors.All() {
			t.Fatal(e)
		}
		out, err := ApplySubstitution(solution.Substitutions, result)
		if err != nil {
			t.Fatal(err)
		}
		if got := TypeString(SimplifyType(out)); got != tc.want {
			t.Fatalf("%s: %s want %s", tc.op, got, tc.want)
		}
	}
	bad := typed(Expr{Kind: "name", Name: "key"}, Type{Kind: "str"})
	_, e := RecordConstraints("field", id, []TypedExpression{receiver, bad}, result)
	if e == nil || *e.Message != "Record field name must be a string expression but is name: key" || e.Location != "Generating record constraints for record,key" {
		t.Fatalf("%+v", e)
	}
	_, e = RecordConstraints("Rec", id, []TypedExpression{bad, value}, result)
	if e == nil || *e.Message != "Record field name must be a name expression but is name: key" {
		t.Fatal(e)
	}
	for _, op := range []string{"with", "field", "fieldNames", "unknown"} {
		if _, e = RecordConstraints(op, id, nil, result); e == nil {
			t.Fatal(op)
		}
	}
	if _, e = RecordConstraints("Rec", id, []TypedExpression{key}, result); e == nil {
		t.Fatal("odd record arity")
	}
}
func TestExpressionDiagnosticPrinting(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"true", "true"}, {"123456789012345678901234567890", "123456789012345678901234567890"},
		{"\"hello\"", "\"hello\""}, {"1+2", "iadd(1, 2)"},
		{"(x,y)=>x", "((x, y) => x)"},
		{"{ pure val x:int=1; x }", "pure val x: int = 1 { x }"},
	} {
		e, err := ParseExpression(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		if got := ExpressionString(e); got != tc.want {
			t.Fatalf("%s != %s", got, tc.want)
		}
	}
}
