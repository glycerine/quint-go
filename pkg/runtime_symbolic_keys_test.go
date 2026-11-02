package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamSymbolicCollectionKeys(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Cases []struct {
			Name, Source, FailureStage              string
			Input                                   Expr
			Output                                  *Expr
			Error                                   *Diagnostic
			Thrown                                  *RuntimeDomainException
			CounterEvaluation, CounterSerialization uint32
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_keys.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			state := &RuntimeObjectHashState{}
			value, err := EvaluatePureExpressionWithRuntimeState(ctx, test.Input, nil, state)
			if state.Counter() != test.CounterEvaluation {
				t.Fatalf("evaluation UID %d, want%d", state.Counter(), test.CounterEvaluation)
			}
			if test.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) {
					t.Fatalf("evaluation %v, want%v", err, test.Error)
				}
				got, _ := json.Marshal(diagnostic)
				want, _ := json.Marshal(test.Error)
				if !bytes.Equal(got, want) {
					t.Fatalf("diagnostic %s, want%s", got, want)
				}
				return
			}
			if err != nil {
				t.Fatalf("evaluation %v", err)
			}
			output, err := SerializeRuntimeValueContext(ctx, value, state)
			if state.Counter() != test.CounterSerialization {
				t.Fatalf("serialization UID%d, want%d", state.Counter(), test.CounterSerialization)
			}
			if test.Thrown != nil {
				var thrown RuntimeDomainException
				if !errors.As(err, &thrown) || thrown != *test.Thrown {
					t.Fatalf("serialization %v, want%v", err, test.Thrown)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.Output == nil {
				t.Fatal("fixture has no expected output")
			}
			got, _ := json.Marshal(output)
			want, _ := json.Marshal(test.Output)
			if !bytes.Equal(got, want) {
				t.Fatalf("source %s\ngot%s\nwant%s", test.Source, got, want)
			}
		})
	}
}
