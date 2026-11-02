package quint

import (
	"errors"
	"testing"
)

func TestUpstreamSetConversions(t *testing.T) {
	testPureEvaluationFixtures(t, "runtime_set_conversion.json")
}

func TestSetConversionsValidationAndOwnership(t *testing.T) {
	value := func(source string) RuntimeValue {
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
	integer := value("1")
	for _, op := range []string{"getOnlyElement"} {
		for _, args := range [][]RuntimeValue{nil, {integer, integer}, {{}}, {integer}} {
			if _, err := EvalSetConversionBuiltin(op, args); err == nil {
				t.Fatal(op, "invalid arguments accepted")
			}
		}
	}
	if _, err := EvalSetConversionBuiltin("missing", []RuntimeValue{integer}); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	for _, source := range []string{"Set()", "Set(1,2)"} {
		_, err := EvalSetConversionBuiltin("getOnlyElement", []RuntimeValue{value(source)})
		var d Diagnostic
		if !errors.As(err, &d) || d.Code != "QNT505" {
			t.Fatal(err)
		}
	}
	singleton := value("Set(Map((2,20),(1,10)))")
	item, err := EvalSetConversionBuiltin("getOnlyElement", []RuntimeValue{singleton})
	if err != nil || item.source.Args[0].Args[0].Int.String() != "2" {
		t.Fatal("singleton source order changed", err)
	}
	copy := item.Expression()
	copy.Args[0].Args[1] = Expr{Kind: "bool", Bool: true}
	original, err := EvalSetConversionBuiltin("getOnlyElement", []RuntimeValue{singleton})
	if err != nil || original.source.Args[0].Args[1].Kind != "int" {
		t.Fatal("singleton expression copy aliases stored value", err)
	}
}
