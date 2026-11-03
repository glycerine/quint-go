package quint

import (
	"context"
	"errors"
	"io"
	"math/big"
	"sync"
	"testing"
)

type closureCancelWriter struct {
	cancel context.CancelFunc
	writes int
}

func (w *closureCancelWriter) Write(data []byte) (int, error) {
	w.writes++
	w.cancel()
	return len(data), nil
}

func TestClosureCancellationAfterFinalSourceCallback(t *testing.T) {
	t.Parallel()
	// A final q::debug write can cancel the embedding while still returning a
	// successful value. There is no next iteration to notice the cancellation.
	for _, source := range []string{
		`Set(1).fold(0, (a, x) => q::debug("cancel", a+x))`,
		`1.to(1).fold(0, (a, x) => q::debug("cancel", a+x))`,
		`List(1).foldl(0, (a, x) => q::debug("cancel", a+x))`,
		`List(1).foldr(0, (x, a) => q::debug("cancel", a+x))`,
	} {
		t.Run(source, func(t *testing.T) {
			expression, err := ParseExpression(source)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &closureCancelWriter{cancel: cancel}
			ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: writer})
			value, err := EvaluatePureExpression(ctx, expression, nil)
			if !errors.Is(err, context.Canceled) || value.Valid() || writer.writes != 1 {
				t.Fatalf("final callback: value=%s writes=%d error=%v", value.String(), writer.writes, err)
			}
		})
	}
}

func TestClosureCanceledReturnRestoresOwner(t *testing.T) {
	t.Parallel()
	expression, err := ParseExpression(`{ val captured=7; (x) => q::debug("call", captured+x) }`)
	if err != nil {
		t.Fatal(err)
	}
	closure, err := EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(big.NewInt(1))})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &closureCancelWriter{cancel: cancel}
	ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: writer})
	value, err := closure.Apply(ctx, []RuntimeValue{one})
	if !errors.Is(err, context.Canceled) || value.Valid() || writer.writes != 1 {
		t.Fatalf("canceled invocation: value=%s writes=%d error=%v", value.String(), writer.writes, err)
	}
	// Cancellation belongs to that invocation, rather than the retained closure,
	// parameter cells, or its owner's active context and debug destination.
	live := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: io.Discard})
	for _, argument := range []int64{2, 3} {
		value, _ := NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(big.NewInt(argument))})
		got, err := closure.Apply(live, []RuntimeValue{value})
		if err != nil || got.expression.Int != IntegerFromBig(big.NewInt(7+argument)) || writer.writes != 1 {
			t.Fatalf("reused closure: value=%s writes=%d error=%v", got.String(), writer.writes, err)
		}
	}
}

func TestClosureCanceledReturnPreservesCallbackError(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{
		&Diagnostic{Code: "QNT507", Message: "callback failed"},
		errors.New("embedding failed"),
		context.DeadlineExceeded,
	} {
		ctx, cancel := context.WithCancel(context.Background())
		closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
			cancel()
			return RuntimeValue{}, failure
		})
		_, err := closure.Apply(ctx, nil)
		cancel()
		if err != failure {
			t.Fatalf("callback error was replaced: got %v, want %v", err, failure)
		}
	}
}

func TestClosureCancellationIndependentConcurrentOwners(t *testing.T) {
	// Each evaluator owns mutable scope/parameter cells. Sharing immutable IR
	// between separately constructed owners must not share cancellation or hooks.
	expression, err := ParseExpression(`{ val captured=7; (x) => q::debug("call", captured+x) }`)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 4
	closures := make([]RuntimeValue, workers)
	for i := range closures {
		closures[i], err = EvaluatePureExpression(context.Background(), expression, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	var ready, finished sync.WaitGroup
	ready.Add(workers)
	finished.Add(workers)
	start := make(chan struct{})
	for i, closure := range closures {
		go func(index int, closure RuntimeValue) {
			defer finished.Done()
			ready.Done()
			<-start
			argument, _ := NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(big.NewInt(int64(index)))})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &closureCancelWriter{cancel: cancel}
			ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: writer})
			if _, err := closure.Apply(ctx, []RuntimeValue{argument}); !errors.Is(err, context.Canceled) {
				t.Errorf("owner %d accepted cancellation: %v", index, err)
			}
			live := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: io.Discard})
			got, err := closure.Apply(live, []RuntimeValue{argument})
			if err != nil || got.expression.Int != IntegerFromBig(big.NewInt(int64(index+7))) || writer.writes != 1 {
				t.Errorf("owner %d leaked invocation state: value=%s writes=%d error=%v", index, got.String(), writer.writes, err)
			}
		}(i, closure)
	}
	ready.Wait()
	close(start)
	finished.Wait()
}
