package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamProjectEvaluation(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Output *Expr
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/runtime_project.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Source, func(t *testing.T) {
			t.Parallel()
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(f.Source, ids)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(modules)
			next := ids.next.String()
			got, err := EvaluatePureModules(context.Background(), modules, ids, "M", "result", nil)
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
			after, _ := json.Marshal(modules)
			if string(before) != string(after) || ids.next.String() != next {
				t.Fatal("input modules or ID generator changed")
			}
		})
	}
}
func TestProjectEvaluationValidation(t *testing.T) {
	for _, source := range []string{
		"module M { val result=1+true }",
		"module M { val result=missing }",
		"module M { import Missing.* val result=1 }",
		"module M { val a=b val b=a val result=a }",
	} {
		ids := NewIDGenerator()
		modules, err := parseModulesWithIDs(source, ids)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = EvaluatePureModules(context.Background(), modules, ids, "M", "result", nil); err == nil {
			t.Fatal("invalid project accepted", source)
		}
	}
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module M { val result=42 }", ids)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EvaluatePureModules(context.Background(), modules, nil, "M", "result", nil); err == nil {
		t.Fatal("nil generator accepted")
	}
	if _, err := EvaluatePureModules(context.Background(), modules, ids, "Missing", "result", nil); err == nil {
		t.Fatal("missing main accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvaluatePureModules(ctx, modules, ids, "M", "result", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
