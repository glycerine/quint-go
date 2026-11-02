package quint

import (
	"context"
	"errors"
	"testing"
)

func TestUpstreamRuntimeRestrictions(t *testing.T) {
	testPureEvaluationFixtures(t, "runtime_restrictions.json")
}

func TestFailValidationAndRestrictionReferences(t *testing.T) {
	for _, args := range [][]Expr{nil, {{Kind: "int"}}, {{Kind: "bool", Bool: true}, {Kind: "bool", Bool: false}}} {
		if _, err := EvaluatePureExpression(context.Background(), Expr{Kind: "app", Opcode: "fail", Args: args}, nil); err == nil {
			t.Fatal("invalid fail accepted")
		}
	}
	for _, opcode := range []string{"always", "next", "unknownRuntimeOperator"} {
		id, _ := ParseID("73")
		_, err := EvaluatePureExpression(context.Background(), Expr{ID: id, Kind: "app", Opcode: opcode, Args: []Expr{{Kind: "bool", Bool: true}}}, nil)
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Reference == nil || diagnostic.Reference.String() != id.String() {
			t.Fatal(opcode, err)
		}
	}
}
