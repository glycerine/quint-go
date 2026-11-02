package tlc

import (
	"context"
	"errors"
	"testing"

	native "github.com/glycerine/tlago/tlc"
)

func TestCancellationGuardPreservesEvalOverloads(t *testing.T) {
	tool := native.NewTool()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installCancellation(ctx, tool)
	state := native.NewEmptyState()
	expression := native.NewValueNode(native.NewIntValue(7))
	for _, args := range [][]any{
		nil, {nil}, {native.EmptyContext}, {native.EmptyContext, state}, {native.EmptyContext, state, native.DoNotRecordCostModel},
		{native.EmptyContext, state, state, native.EvalClear}, {native.EmptyContext, state, state, native.EvalClear, native.DoNotRecordCostModel}, {nil, nil, nil, native.EvalClear},
	} {
		value, err := tool.Eval(expression, args...)
		if err != nil || value == nil || value.String() != "7" {
			t.Fatal(value, err, args)
		}
	}
	cancel()
	if _, err := tool.Eval(expression); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCancellationGuardComposesCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tool := native.NewTool()
	state := native.NewEmptyState()
	evaluated, initial, next := 0, 0, 0
	tool.EvalFunc = func(*native.Tool, native.SemanticNode, ...any) (native.Value, error) {
		evaluated++
		return native.NewIntValue(7), nil
	}
	tool.GetInitStatesFunc = func(_ *native.Tool, f *native.StateFunctor) error {
		initial++
		_, err := f.AddElement(state)
		return err
	}
	tool.GetNextStatesWithFunctorFn = func(_ *native.Tool, f *native.NextStateFunctor, _ *native.TLCStateMut) (bool, error) {
		next++
		_, err := f.AddNextElement(state, nil, state)
		return false, err
	}
	installCancellation(ctx, tool)
	value, err := tool.Eval(native.NewNumeralNode(7))
	if err != nil || value == nil || value.String() != "7" || evaluated != 1 {
		t.Fatal(value, err, evaluated)
	}
	added := 0
	initialFunctor := native.NewStateFunctor(func(*native.TLCStateMut) (any, error) { added++; return nil, nil })
	initialFunctor.SetElementFunc = func(*native.TLCStateMut) (any, error) { added++; return nil, nil }
	guarded := guardStateFunctor(ctx, initialFunctor)
	if _, err = guarded.SetElement(state); err != nil || added != 1 {
		t.Fatal(err, added)
	}
	if err = tool.GetInitStates(initialFunctor); err != nil || initial != 1 || added != 2 {
		t.Fatal(err, initial, added)
	}
	nextFunctor := native.NewNextStateFunctor(func(*native.TLCStateMut, *native.Action, *native.TLCStateMut) (any, error) { added++; return nil, nil })
	if _, err = tool.GetNextStatesWithFunctor(nextFunctor, state); err != nil || next != 1 || added != 3 {
		t.Fatal(err, next, added)
	}
	cancel()
	if err = tool.GetInitStates(initialFunctor); !errors.Is(err, context.Canceled) || initial != 1 {
		t.Fatal(err, initial)
	}
	if _, err = tool.GetNextStatesWithFunctor(nextFunctor, state); !errors.Is(err, context.Canceled) || next != 1 {
		t.Fatal(err, next)
	}
	if _, err = guarded.AddElement(state); !errors.Is(err, context.Canceled) || added != 3 {
		t.Fatal(err, added)
	}
	if _, err = guarded.SetElement(state); !errors.Is(err, context.Canceled) || added != 3 {
		t.Fatal(err, added)
	}
}
