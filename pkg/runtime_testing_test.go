package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamRuntimeTesting(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name       string
		Module     Module
		MaxSamples int
		Seed       string
		Metadata   bool
		Result     struct {
			Name     string
			Status   string
			Errors   []Diagnostic
			Seed     Integer
			NSamples int
		}
		Selected []Expr
		RNG      string
		Record   Expr
		Trace    []Expr
	}
	data, err := os.ReadFile("testdata/parser/runtime_testing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			seed, err := ParseInteger(f.Seed)
			if err != nil {
				t.Fatal(err)
			}
			e, err := NewModuleEvaluator(f.Module, NewRNG(seed), f.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			got, err := e.Test(context.Background(), "example", f.MaxSamples)
			if err != nil {
				t.Fatal(err)
			}
			want := f.Result
			if got.Name != want.Name || got.Status != want.Status || got.Samples != want.NSamples || got.Seed != want.Seed {
				t.Fatalf("result got %+v want %+v", got, want)
			}
			if len(got.Errors) != len(want.Errors) {
				t.Fatal(got.Errors, want.Errors)
			}
			for i, d := range want.Errors {
				if got.Errors[i].Code != d.Code || got.Errors[i].Message != d.Message {
					t.Fatal(got.Errors[i], d)
				}
				if d.Code == "QNT511" && (got.Errors[i].Reference == nil || *got.Errors[i].Reference != *d.Reference) {
					t.Fatal("test definition reference", got.Errors[i], d)
				}
			}
			equalStates := func(label string, states []RuntimeValue, expressions []Expr) {
				t.Helper()
				if len(states) != len(expressions) {
					t.Fatal(label, "length", len(states), len(expressions))
				}
				for i, expression := range expressions {
					value, err := NewRuntimeValue(expression)
					if err != nil || !states[i].Equal(value) {
						t.Fatal(label, i, states[i].String(), value.String(), err)
					}
				}
			}
			equalStates("selected", got.States, f.Selected)
			equalStates("active", e.Trace.States(), f.Trace)
			record, err := e.Storage.Record()
			expected, err2 := NewRuntimeValue(f.Record)
			if err != nil || err2 != nil || !record.Equal(expected) || e.RNG.State().String() != f.RNG {
				t.Fatal(record.String(), expected.String(), e.RNG.State().String(), f.RNG, err, err2)
			}
		})
	}
}

func TestRuntimeTestingValidationZeroAndOwnership(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action example=x'=1 }")
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewModuleEvaluator(modules[0], nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Test(context.Background(), "missing", 1); err == nil {
		t.Fatal("missing name accepted")
	}
	if _, err = e.Test(context.Background(), "example", -1); err == nil {
		t.Fatal("negative samples accepted")
	}
	got, err := e.Test(context.Background(), "example", 5)
	if err != nil || got.Status != "passed" || got.Samples != 1 || len(got.States) != 1 {
		t.Fatal(got, err)
	}
	before, err := e.Storage.Record()
	if err != nil {
		t.Fatal(err)
	}
	got.States[0] = RuntimeValue{}
	if !e.Trace.States()[0].Valid() {
		t.Fatal("result aliases active trace")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = e.Test(ctx, "example", 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(e.Trace.States()) != 1 {
		t.Fatal("pre-cancel mutated trace")
	}
	zero, err := e.Test(context.Background(), "example", 0)
	if err != nil || zero.Status != "passed" || zero.Samples != 0 || len(zero.States) != 0 || len(e.Trace.States()) != 0 {
		t.Fatal(zero, err)
	}
	after, err := e.Storage.Record()
	if err != nil || !before.Equal(after) {
		t.Fatal("zero samples reset current state", err)
	}
}

func TestRuntimeTestingCancellationBetweenSamples(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action example=any { x'=1,x'=2 } }")
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	limited := &simulationCancelContext{Context: ctx, remaining: 100, cancel: cancel}
	result, err := e.Test(limited, "example", 1000)
	if !errors.Is(err, context.Canceled) || result.Samples >= 1000 {
		t.Fatal(result, err)
	}
}

func TestRuntimeTestingFailingSeedReplay(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action example=(any { x'=0,x'=1 }).then(x==1) }")
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := e.Test(context.Background(), "example", 10)
	if err != nil || failed.Status != "failed" || failed.Samples != 2 || failed.Seed.String() != "1" {
		t.Fatal(failed, err)
	}
	replay, err := NewModuleEvaluator(modules[0], NewRNG(failed.Seed), false)
	if err != nil {
		t.Fatal(err)
	}
	again, err := replay.Test(context.Background(), "example", 1)
	if err != nil || again.Status != failed.Status || again.Seed != failed.Seed || len(again.States) != len(failed.States) {
		t.Fatal(again, err)
	}
	for i, state := range again.States {
		if !state.Equal(failed.States[i]) {
			t.Fatal("failed seed did not reproduce state", i)
		}
	}
	if replay.RNG.State() != e.RNG.State() {
		t.Fatal("failed seed did not reproduce draws")
	}
}
