package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamModuleEvaluation(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Module Module
		Output *Expr
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/runtime_module.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Source, func(t *testing.T) {
			before, _ := json.Marshal(f.Module)
			got, err := EvaluatePureDefinition(context.Background(), f.Module, "result", nil)
			if f.Error != nil {
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != f.Error.Code || d.Message != f.Error.Message {
					t.Fatal(err, f.Error)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				want, err := NewRuntimeValue(*f.Output)
				if err != nil || !got.Equal(want) {
					t.Fatal(got.String(), want.String(), err)
				}
			}
			after, _ := json.Marshal(f.Module)
			if string(before) != string(after) {
				t.Fatal("module changed")
			}
		})
	}
}
func TestModuleEvaluationCallsAndDiagnostics(t *testing.T) {
	parse := func(source string) Module {
		t.Helper()
		modules, err := ParseModules(source)
		if err != nil {
			t.Fatal(err)
		}
		return modules[0]
	}
	module := parse("module M { def inc(x)=x+1 val result=42 }")
	n, _ := ParseInteger("41")
	argument, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	got, err := EvaluatePureDefinition(context.Background(), module, "inc", []RuntimeValue{argument})
	if err != nil || got.String() != "42" {
		t.Fatal(got.String(), err)
	}
	closure, err := EvaluatePureDefinition(context.Background(), module, "inc", nil)
	if err != nil || closure.closure == nil {
		t.Fatal(err)
	}
	got, err = closure.Apply(context.Background(), []RuntimeValue{argument})
	if err != nil || got.String() != "42" {
		t.Fatal(got.String(), err)
	}
	if _, err = EvaluatePureDefinition(context.Background(), module, "result", []RuntimeValue{}); err == nil {
		t.Fatal("called a value")
	}
	for _, source := range []string{
		"module M { val other=1 }",
		"module M { val result=missing }",
		"module M { val a=b val b=a val result=a }",
		"module M { import Other.* val result=1 }",
	} {
		if _, err := EvaluatePureDefinition(context.Background(), parse(source), "result", nil); err == nil {
			t.Fatal("invalid module accepted", source)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = EvaluatePureDefinition(ctx, module, "result", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
