package quint

import "testing"

func TestSetValidationAndOwnership(t *testing.T) {
	makeValue := func(source string) RuntimeValue {
		t.Helper()
		e, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		v, err := NewRuntimeValue(e)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	a, b := makeValue("Set(1,2)"), makeValue("Set(2,3)")
	beforeA, beforeB := a.String(), b.String()
	for _, op := range []string{"union", "intersect", "exclude", "subseteq"} {
		result, err := EvalSetBuiltin(op, []RuntimeValue{a, b})
		if err != nil {
			t.Fatal(op, err)
		}
		exported := result.Expression()
		if len(exported.Args) > 0 {
			exported.Args[0] = Expr{Kind: "str", Str: "changed"}
		}
		if a.String() != beforeA || b.String() != beforeB {
			t.Fatal("input mutation", op)
		}
	}
	scalar := makeValue("1")
	badNested := makeValue("Set(1)")
	for _, c := range []struct {
		op   string
		args []RuntimeValue
	}{
		{"unknown", nil}, {"size", nil}, {"size", []RuntimeValue{{}}}, {"size", []RuntimeValue{scalar}}, {"union", []RuntimeValue{a, scalar}}, {"flatten", []RuntimeValue{badNested}}, {"in", []RuntimeValue{scalar, scalar}},
	} {
		if _, err := EvalSetBuiltin(c.op, c.args); err == nil {
			t.Fatal("invalid call accepted", c.op)
		}
	}
	// Exact identity stays independent of collation ties in nested Unicode sets.
	left := makeValue("Set(Set(\"é\",\"é\"))")
	right := makeValue("Set(Set(\"é\",\"é\"))")
	result, err := EvalSetBuiltin("intersect", []RuntimeValue{left, right})
	if err != nil || !result.Equal(left) {
		t.Fatal(result.String(), err)
	}
}
