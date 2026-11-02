package quint

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUpstreamRanges(t *testing.T) { testPureEvaluationFixtures(t, "runtime_range.json") }
func TestRangeValidationAndCancellation(t *testing.T) {
	integer := func(s string) RuntimeValue {
		n, _ := ParseInteger(s)
		v, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
		return v
	}
	boolean, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	for _, args := range [][]RuntimeValue{nil, {integer("1")}, {integer("1"), boolean}, {RuntimeValue{}, integer("2")}, {integer("1" + strings.Repeat("0", 400)), integer("0")}} {
		if _, err := EvalRangeBuiltin(context.Background(), args); err == nil {
			t.Fatal("invalid arguments accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvalRangeBuiltin(ctx, []RuntimeValue{integer("0"), integer("100")}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
