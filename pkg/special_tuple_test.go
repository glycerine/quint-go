package quint

import "testing"

func TestTupleSpecialConstraints(t *testing.T) {
	id, _ := ParseID("1")
	result := Type{Kind: "var", Name: "r"}
	for _, args := range [][]TypedExpression{nil, {{Type: Type{Kind: "int"}}, {Type: Type{Kind: "str"}}}} {
		cs := TupleConstructorConstraints(id, args, result)
		if cs[0].Types[0].Fields.Kind != "row" || len(cs[0].Types[0].Fields.Fields) != len(args) {
			t.Fatal("constructor shape")
		}
		solution, err := (TypeUnifier{}).SolveConstraint(cs[0])
		if err != nil {
			t.Fatal(err)
		}
		out, err := ApplySubstitution(solution.Substitutions, result)
		if err != nil || out.Kind != "tup" {
			t.Fatalf("%v %+v", err, out)
		}
	}
	tuple, _ := ParseType("(int,str,bool)")
	for _, test := range []struct{ index, want string }{{"1", "int"}, {"2", "str"}, {"3", "bool"}} {
		n, _ := ParseInteger(test.index)
		cs, e := TupleItemConstraints(id, []TypedExpression{{Expr: Expr{Kind: "name", Name: "t"}, Type: tuple}, {Expr: Expr{Kind: "int", Int: n}, Type: Type{Kind: "int"}}}, result)
		if e != nil {
			t.Fatal(e)
		}
		solved, err := (TypeUnifier{}).SolveConstraint(cs[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range solved.Errors.All() {
			t.Fatal(e)
		}
		out, err := ApplySubstitution(solved.Substitutions, result)
		if err != nil || out.Kind != test.want {
			t.Fatalf("%v %+v", err, out)
		}
	}
}
func TestTupleItemDiagnosticsAndUnusualIndices(t *testing.T) {
	id, _ := ParseID("1")
	r := Type{Kind: "var", Name: "r"}
	tuple, _ := ParseType("(int,str)")
	args := []TypedExpression{{Expr: Expr{Kind: "name", Name: "t"}, Type: tuple}, {Expr: Expr{Kind: "name", Name: "n"}}}
	if _, e := TupleItemConstraints(id, args, r); e == nil || *e.Message != "Tup field index must be an int expression but is name: n" {
		t.Fatal(e)
	}
	for _, n := range []string{"0", "-1", "999999999999999999999999999"} {
		v, _ := ParseInteger(n)
		args[1].Expr = Expr{Kind: "int", Int: v}
		cs, e := TupleItemConstraints(id, args, r)
		if e != nil {
			t.Fatal(e)
		}
		if len(cs[0].Types[1].Fields.Fields) != 1 {
			t.Fatal("unexpected prefix")
		}
		solved, err := (TypeUnifier{}).SolveConstraint(cs[0])
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for range solved.Errors.All() {
			count++
		}
		if count == 0 {
			t.Fatal("invalid projection accepted")
		}
	}
	large, _ := ParseInteger("1000002")
	args[1].Expr = Expr{Kind: "int", Int: large}
	if _, e := TupleItemConstraints(id, args, r); e == nil {
		t.Fatal("prefix limit")
	}
	if _, e := TupleItemConstraints(id, nil, r); e == nil {
		t.Fatal("arity")
	}
}
