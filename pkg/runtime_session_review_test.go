package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type runtimeSessionReviewFixture struct {
	Arities []struct {
		Name      string
		Input     Expr
		Arguments []Expr
		Error     Diagnostic
	}
	Literals []struct {
		Name, Mode    string
		Input, Output Expr
		SameObject    bool
	}
	Incremental struct {
		Source, Unrelated string
		SameObject        bool
	}
	ActionModule Module
	Actions      []struct {
		Op, Name, Action string
		Output           *Expr
		SameAsFirst      bool
	}
}

func loadRuntimeSessionReview(t *testing.T) runtimeSessionReviewFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/runtime_session_review.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture runtimeSessionReviewFixture
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// An incremental table update must keep the builder's compiled definition
// identity when an existing declaration keeps its ID.
func TestREPLCompiledDefinitionSurvivesUnrelatedInputs(t *testing.T) {
	t.Parallel()
	reference := loadRuntimeSessionReview(t).Incremental
	ctx := context.Background()
	session := NewREPLSession()
	if _, err := session.Evaluate(ctx, reference.Source); err != nil {
		t.Fatal(err)
	}
	first, err := session.Evaluate(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	declaration, _ := session.runtime.definitions.Get2("f")
	if _, err = session.Evaluate(ctx, reference.Unrelated); err != nil {
		t.Fatal(err)
	}
	repeated, err := session.Evaluate(ctx, "f")
	if err != nil {
		t.Fatal(err)
	}
	nextDeclaration, _ := session.runtime.definitions.Get2("f")
	if declaration.ID != nextDeclaration.ID {
		t.Fatalf("fixture changed definition identity: %s != %s", declaration.ID.String(), nextDeclaration.ID.String())
	}
	if sameRuntimeObject(first.Value, repeated.Value) != reference.SameObject {
		t.Fatal("incremental update rebuilt an unchanged compiled lambda")
	}
}

func TestUpstreamCompiledLiteralIdentity(t *testing.T) {
	t.Parallel()
	reference := loadRuntimeSessionReview(t)
	integer := func(text string) RuntimeValue {
		t.Helper()
		number, err := ParseInteger(text)
		if err != nil {
			t.Fatal(err)
		}
		value, err := NewRuntimeValue(Expr{Kind: "int", Int: number})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, fixture := range reference.Literals {
		t.Run(fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			session := newRuntimeEvaluation(nil, nil, nil, nil, nil)
			expression := copyRuntimeExpression(fixture.Input)
			first, err := session.evaluateOwned(ctx, expression)
			if err != nil {
				t.Fatal(err)
			}
			second, err := session.evaluateOwned(ctx, expression)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.Mode == "invoke" {
				first, err = first.Apply(ctx, []RuntimeValue{integer("1")})
				if err != nil {
					t.Fatal(err)
				}
				second, err = second.Apply(ctx, []RuntimeValue{integer("2")})
				if err != nil {
					t.Fatal(err)
				}
			}
			expected, err := NewRuntimeValue(fixture.Output)
			if err != nil || !first.Equal(expected) || !second.Equal(expected) || sameRuntimeObject(first, second) != fixture.SameObject {
				t.Fatal("compiled literal identity", first.String(), second.String(), expected.String(), err)
			}
		})
	}
}

func TestUpstreamClosureArityDiagnostics(t *testing.T) {
	t.Parallel()
	reference := loadRuntimeSessionReview(t)
	for _, fixture := range reference.Arities {
		t.Run(fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			closure, err := EvaluatePureExpression(ctx, fixture.Input, nil)
			if err != nil {
				t.Fatal(err)
			}
			args := make([]RuntimeValue, len(fixture.Arguments))
			for i, arg := range fixture.Arguments {
				args[i], err = NewRuntimeValue(arg)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err = closure.Apply(ctx, args)
			var diagnostic Diagnostic
			if !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message || diagnostic.Reference != nil {
				t.Fatal("direct closure arity", err, fixture.Error)
			}
		})
	}
}

func TestUpstreamActionClosureMetadataAndIdentity(t *testing.T) {
	t.Parallel()
	fixture := loadRuntimeSessionReview(t)
	evaluator, err := NewModuleEvaluator(fixture.ActionModule, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var first RuntimeValue
	zero, _ := NewRuntimeValue(Expr{Kind: "int"})
	for index, step := range fixture.Actions {
		var value RuntimeValue
		switch step.Op {
		case "clear":
			evaluator.Storage.ClearMetadata()
		case "invokeSaved":
			value, err = first.Apply(ctx, []RuntimeValue{zero})
		case "get":
			value, err = evaluator.Evaluate(ctx, step.Name, nil)
			if !first.Valid() {
				first = value
			}
		default:
			t.Fatal(step.Op)
		}
		if err != nil || evaluator.Storage.ActionTaken != step.Action {
			t.Fatal(index, "action metadata", evaluator.Storage.ActionTaken, step.Action, err)
		}
		if step.Output != nil && (ExpressionString(value.Expression()) != ExpressionString(*step.Output) || sameRuntimeObject(value, first) != step.SameAsFirst) {
			t.Fatal(index, "action value", value.String(), ExpressionString(*step.Output))
		}
	}
}
