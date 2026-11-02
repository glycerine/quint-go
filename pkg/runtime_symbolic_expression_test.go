package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type symbolicExpressionCase struct {
	Name, Source      string
	Input             Expr
	Output            *Expr
	Error             *Diagnostic
	Thrown            *RuntimeDomainException
	Counter           uint32
	SharedObjectState bool
}

func assertSymbolicExpression(t *testing.T, test symbolicExpressionCase) {
	t.Helper()
	ctx := context.Background()
	before, err := json.Marshal(test.Input)
	if err != nil {
		t.Fatal(err)
	}
	// Most cases exercise the ordinary source evaluator. Threshold cases
	// explicitly share its object sequence with checked serialization, including
	// cases that consume raw-object hashes before their result is displayed.
	state := &RuntimeObjectHashState{}
	var value RuntimeValue
	var evaluationError error
	if test.SharedObjectState {
		value, evaluationError = EvaluatePureExpressionWithRuntimeState(ctx, test.Input, nil, state)
	} else {
		value, evaluationError = EvaluatePureExpression(ctx, test.Input, nil)
	}
	after, err := json.Marshal(test.Input)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("evaluation changed the source IR", err)
	}
	if test.Error != nil {
		if state.Counter() != test.Counter {
			t.Fatalf("source %s: object counter %d, want %d", test.Source, state.Counter(), test.Counter)
		}
		var diagnostic Diagnostic
		if !errors.As(evaluationError, &diagnostic) {
			t.Fatalf("source %s: got %v, want diagnostic %#v", test.Source, evaluationError, test.Error)
		}
		got, _ := json.Marshal(diagnostic)
		want, _ := json.Marshal(test.Error)
		if !bytes.Equal(got, want) {
			t.Fatalf("source %s: diagnostic %s, want %s", test.Source, got, want)
		}
		return
	}
	if evaluationError != nil {
		t.Fatalf("source %s: evaluation %v", test.Source, evaluationError)
	}
	output, serializationError := SerializeRuntimeValueContext(ctx, value, state)
	if state.Counter() != test.Counter {
		t.Fatalf("source %s: object counter %d, want %d", test.Source, state.Counter(), test.Counter)
	}
	if test.Thrown != nil {
		var thrown RuntimeDomainException
		if !errors.As(serializationError, &thrown) || thrown != *test.Thrown {
			t.Fatalf("source %s: serialization %v, want %#v", test.Source, serializationError, *test.Thrown)
		}
		return
	}
	if serializationError != nil {
		t.Fatalf("source %s: serialization %v", test.Source, serializationError)
	}
	if test.Output == nil {
		t.Fatal("saved source case has no expected outcome")
	}
	got, _ := json.Marshal(output)
	want, _ := json.Marshal(test.Output)
	if !bytes.Equal(got, want) {
		t.Fatalf("source %s:\ngot  %s\nwant %s", test.Source, got, want)
	}
}

func TestUpstreamSymbolicSourceExpressions(t *testing.T) {
	t.Parallel()
	var fixture struct{ Cases []symbolicExpressionCase }
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_expression.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			assertSymbolicExpression(t, test)
		})
	}
}

func TestHistoricalSymbolicSourceCorpus(t *testing.T) {
	t.Parallel()
	// Replay the original pending source cases through actual default dispatch.
	// Their library-domain snapshots are covered independently; the historical
	// filename does not substitute for a successful source evaluation here.
	var fixture struct {
		Expressions []struct {
			Source     string
			Input      Expr
			Value      *Expr
			Diagnostic *Diagnostic
			Thrown     *RuntimeDomainException
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_pending.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Expressions {
		t.Run(test.Source, func(t *testing.T) {
			assertSymbolicExpression(t, symbolicExpressionCase{
				Source: test.Source, Input: test.Input, Output: test.Value,
				Error: test.Diagnostic, Thrown: test.Thrown,
			})
		})
	}
}
