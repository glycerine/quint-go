package quint

import (
	"fmt"
	"testing"
)

func TestApplicationConstraintArity(t *testing.T) {
	id, _ := ParseID("1")
	r := Type{Kind: "var", Name: "r"}
	for _, tc := range []struct {
		op, expected string
		count        int
	}{
		{"Rec", "even number of", 1}, {"field", "2", 1}, {"fieldNames", "1", 0},
		{"with", "3", 2}, {"item", "2", 3}, {"variant", "2", 1}, {"matchVariant", "odd number of", 0},
	} {
		cs, es := ApplicationConstraints(tc.op, id, make([]TypedExpression, tc.count), r, Type{})
		if cs != nil || len(es) != 1 || es[0].Location != "Checking arity for application of "+tc.op || *es[0].Message != fmt.Sprintf("Operator expects %s arguments but was given %d", tc.expected, tc.count) {
			t.Fatalf("%s %+v", tc.op, es)
		}
	}
}
func TestApplicationConstraintStandardAndSpecial(t *testing.T) {
	id, _ := ParseID("1")
	r := Type{Kind: "var", Name: "r"}
	signature, _ := ParseType("(int,int)=>bool")
	args := []TypedExpression{{Type: Type{Kind: "int"}}, {Type: Type{Kind: "int"}}}
	cs, es := ApplicationConstraints("compare", id, args, r, signature)
	if len(es) != 0 || len(cs) != 1 || cs[0].SourceID != id {
		t.Fatal(cs, es)
	}
	solution, err := (TypeUnifier{}).SolveConstraint(cs[0])
	if err != nil {
		t.Fatal(err)
	}
	out, err := ApplySubstitution(solution.Substitutions, r)
	if err != nil || out.Kind != "bool" {
		t.Fatal(out, err)
	}
	for _, op := range []string{"Tup", "Rec"} {
		cs, es = ApplicationConstraints(op, id, nil, r, Type{})
		if len(es) != 0 || len(cs) != 1 {
			t.Fatal(op, cs, es)
		}
	}
	// Valid arity still propagates special-rule validation errors.
	cs, es = ApplicationConstraints("field", id, []TypedExpression{{Type: Type{Kind: "var", Name: "rec"}}, {Expr: Expr{Kind: "name", Name: "key"}}}, r, Type{})
	if cs != nil || len(es) != 1 || es[0].Location == "Checking arity for application of field" {
		t.Fatal(cs, es)
	}
}
