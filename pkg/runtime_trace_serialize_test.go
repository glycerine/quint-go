package quint

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSerializeRuntimeTraceSharesObjectHistory(t *testing.T) {
	t.Parallel()
	expression, err := ParseExpression(`Map((0,1),(1,1),(2,1),(3,1),(4,1),(5,1),(6,1),("é",1),("é",1))`)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	states := make([]RuntimeValue, 8)
	for i := range states {
		states[i] = value
	}
	objects := &RuntimeObjectHashState{}
	got, err := SerializeRuntimeTraceContext(context.Background(), states, objects)
	if err != nil || objects.Counter() != 144 {
		t.Fatal("shared history", objects.Counter(), err)
	}
	seen := map[string]bool{}
	for _, expression := range got {
		encoded, err := json.Marshal(expression)
		if err != nil {
			t.Fatal(err)
		}
		seen[string(encoded)] = true
	}
	if len(seen) != 2 {
		t.Fatal("equal printed strings lost reference hash order", len(seen))
	}
	fresh, err := SerializeRuntimeTraceContext(context.Background(), states, nil)
	if err != nil || !reflect.DeepEqual(got, fresh) {
		t.Fatal("nil history must create one sequence for the whole trace", err)
	}
}

func TestSerializeRuntimeTraceContextBoundaries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	value := func(domain RuntimeDomain) RuntimeValue {
		t.Helper()
		result, err := RuntimeValueFromDomain(domain)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	finite := value(NewRuntimeInterval(domainFixtureInteger(t, "9"), domainFixtureInteger(t, "12")))
	objects := &RuntimeObjectHashState{}
	states, err := SerializeRuntimeTraceContext(ctx, []RuntimeValue{finite, finite}, objects)
	if err != nil || len(states) != 2 || ExpressionString(states[0]) != "Set(9, 10, 11, 12)" {
		t.Fatal(states, err)
	}
	states[0].Args[0] = Expr{Kind: "str", Str: "changed"}
	if ExpressionString(states[1]) != "Set(9, 10, 11, 12)" || finite.domain == nil {
		t.Fatal("serialized states share expressions or materialized their retained input")
	}
	if empty, err := SerializeRuntimeTraceContext(ctx, nil, nil); err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("empty trace", empty, err)
	}
	if _, err := SerializeRuntimeTraceContext(ctx, []RuntimeValue{{}}, nil); err == nil || !strings.Contains(err.Error(), "invalid runtime value") {
		t.Fatal("invalid state", err)
	}
	infinite, err := NewRuntimeInfiniteSet("Int")
	if err != nil {
		t.Fatal(err)
	}
	powerset, err := NewRuntimePowerset(infinite)
	if err != nil {
		t.Fatal(err)
	}
	_, err = SerializeRuntimeTraceContext(ctx, []RuntimeValue{value(powerset)}, objects)
	var exception RuntimeDomainException
	if !errors.As(err, &exception) || exception.Name != "Error" || exception.Message != "Either state is Left" {
		t.Fatal("infinite compound display", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if states, err := SerializeRuntimeTraceContext(canceled, nil, objects); !errors.Is(err, context.Canceled) || states != nil {
		t.Fatal("pre-canceled empty trace", states, err)
	}

	// Cancellation must interrupt finite enumeration rather than attempting to
	// allocate the interval before the caller can regain control.
	huge := value(NewRuntimeInterval(Integer{}, domainFixtureInteger(t, "1000000000000000000000000000000000000")))
	active, cancel := context.WithCancel(ctx)
	defer cancel()
	bounded := &simulationCancelContext{Context: active, remaining: 10, cancel: cancel}
	if states, err := SerializeRuntimeTraceContext(bounded, []RuntimeValue{huge}, nil); !errors.Is(err, context.Canceled) || states != nil {
		t.Fatal("cancellation during enumeration", states, err)
	}
}

type traceSnapshotCancelListener struct {
	ExecutionListener
	cancel context.CancelFunc
	calls  int
}

func (listener *traceSnapshotCancelListener) OnRunReturn(result RuntimeValue, err error, trace []RuntimeValue) {
	listener.ExecutionListener.OnRunReturn(result, err, trace)
	listener.calls++
	listener.cancel()
}

func TestTraceSnapshotDriversPreserveCancellation(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules("module M { var x:int->int action init=x'=0.to(8).mapBy(i=>i) action step=x'=x val inv=true action example=init }")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"simulation", "test"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			evaluator, err := NewModuleEvaluator(modules[0], nil, false)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listener := &traceSnapshotCancelListener{ExecutionListener: NewTraceRecorder(0, evaluator.RNG), cancel: cancel}
			ctx = WithExecutionListener(ctx, listener)
			if mode == "simulation" {
				_, err = evaluator.Simulate(ctx, SimulationOptions{Init: "init", Step: "step", Invariant: "inv", MaxSamples: 1, NumberOfTraces: 1})
			} else {
				_, err = evaluator.Test(ctx, "example", 1)
			}
			if !errors.Is(err, context.Canceled) || listener.calls != 1 || len(mustStates(&evaluator.Trace)) != 1 || evaluator.ObjectHashState().Counter() != 0 {
				t.Fatal("cancellation before selected-state snapshot", err, listener.calls, len(mustStates(&evaluator.Trace)), evaluator.ObjectHashState().Counter())
			}
		})
	}
}
