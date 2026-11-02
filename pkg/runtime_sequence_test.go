package quint

import "testing"

func TestSequenceValidationAndOwnership(t *testing.T) {
	listExpr, _ := ParseExpression("List(1,2)")
	list, _ := NewRuntimeValue(listExpr)
	n, _ := ParseInteger("0")
	zero, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	replacement, _ := NewRuntimeValue(Expr{Kind: "str", Str: "new"})
	before := list.String()
	result, err := EvalSequenceBuiltin("replaceAt", []RuntimeValue{list, zero, replacement})
	if err != nil || result.String() == before || list.String() != before {
		t.Fatal(result.String(), err)
	}
	for _, c := range []struct {
		op   string
		args []RuntimeValue
	}{
		{"unknown", nil}, {"head", nil}, {"head", []RuntimeValue{{}}}, {"head", []RuntimeValue{zero}}, {"nth", []RuntimeValue{list, replacement}}, {"concat", []RuntimeValue{list, zero}}, {"slice", []RuntimeValue{list, zero, replacement}},
	} {
		if _, err := EvalSequenceBuiltin(c.op, c.args); err == nil {
			t.Fatal("invalid call accepted", c.op)
		}
	}
	record, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "Rec"})
	if _, err := EvalSequenceBuiltin("field", []RuntimeValue{record, zero}); err == nil {
		t.Fatal("nonstring field accepted")
	}
	huge, _ := ParseInteger("99999999999999999999999999999999999999")
	index, _ := NewRuntimeValue(Expr{Kind: "int", Int: huge})
	if _, err := EvalSequenceBuiltin("nth", []RuntimeValue{list, index}); err == nil {
		t.Fatal("huge index accepted")
	}
	if _, err := EvalSequenceBuiltin("slice", []RuntimeValue{list, index, index}); err == nil {
		t.Fatal("huge slice accepted")
	}
}
