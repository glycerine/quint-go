package tlc

import (
	"context"

	native "github.com/glycerine/tlago/tlc"
)

// Stop wakes TLC's worker queue. Functor guards additionally interrupt a single
// initial/next-state enumeration, which can otherwise run long after Stop.
// Parsing and individual native builtin evaluations remain cooperative.
func installCancellation(ctx context.Context, tool *native.Tool) {
	evaluate := tool.EvalFunc
	tool.EvalFunc = func(t *native.Tool, expression native.SemanticNode, args ...any) (native.Value, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if evaluate != nil {
			return evaluate(t, expression, args...)
		}
		// Tool.Eval validates these public overloads before invoking EvalFunc.
		// Reproduce their defaults while calling EvalImpl without recursion.
		con, s0, s1, control, cm := native.EmptyContext, native.EmptyState, native.EmptyState, native.EvalClear, native.CostModel{}
		if len(args) > 0 {
			con = nil
			if args[0] != nil {
				con = args[0].(*native.Context)
			}
		}
		if len(args) > 1 {
			s0 = nil
			if args[1] != nil {
				s0 = args[1].(*native.TLCStateMut)
			}
		}
		if len(args) == 3 {
			cm = args[2].(native.CostModel)
		}
		if len(args) >= 4 {
			s1 = nil
			if args[2] != nil {
				s1 = args[2].(*native.TLCStateMut)
			}
			control = args[3].(int)
		}
		if len(args) == 5 {
			cm = args[4].(native.CostModel)
		}
		return t.EvalImpl(expression, con, s0, s1, control, cm)
	}
	initial := tool.GetInitStatesFunc
	tool.GetInitStatesFunc = func(t *native.Tool, functor *native.StateFunctor) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		wrapped := guardStateFunctor(ctx, functor)
		if initial != nil {
			return initial(t, &wrapped)
		}
		return t.GetInitStatesImpl(&wrapped)
	}
	next := tool.GetNextStatesWithFunctorFn
	tool.GetNextStatesWithFunctorFn = func(t *native.Tool, functor *native.NextStateFunctor, state *native.TLCStateMut) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		wrapped := *functor
		wrapped.StateFunctor = guardStateFunctor(ctx, &functor.StateFunctor)
		wrapped.AddNextElementFunc = func(current *native.TLCStateMut, action *native.Action, successor *native.TLCStateMut) (any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return functor.AddNextElement(current, action, successor)
		}
		wrapped.HaltFunc = func() bool { return ctx.Err() != nil || functor.Halt() }
		wrapped.ShouldHaltFunc = func() bool { return ctx.Err() != nil || functor.ShouldHalt() }
		if next != nil {
			return next(t, &wrapped, state)
		}
		for _, action := range t.GetActions() {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			if _, err := t.GetNextStatesForAction(&wrapped, state, action); err != nil {
				return true, err
			}
		}
		return false, nil
	}
}

func guardStateFunctor(ctx context.Context, functor *native.StateFunctor) native.StateFunctor {
	wrapped := *functor
	wrapped.AddElementFunc = func(state *native.TLCStateMut) (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return functor.AddElement(state)
	}
	wrapped.SetElementFunc = func(state *native.TLCStateMut) (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return functor.SetElement(state)
	}
	return wrapped
}

func isolateGlobals() func() {
	native.Globals.Lock()
	continuation, probabilistic := native.Globals.Continuation, native.Globals.Probabilistic
	workers, dfid := native.Globals.NumWorkers, native.Globals.DFIDMax
	metadata, tool, start := native.Globals.MetaDir, native.Globals.Tool, native.Globals.StartTime
	checker := native.Globals.MainChecker
	native.Globals.Continuation = false
	native.Globals.Probabilistic = false
	native.Globals.Unlock()
	return func() {
		native.Globals.Lock()
		native.Globals.Continuation = continuation
		native.Globals.Probabilistic = probabilistic
		native.Globals.NumWorkers = workers
		native.Globals.DFIDMax = dfid
		native.Globals.MetaDir = metadata
		native.Globals.Tool = tool
		native.Globals.StartTime = start
		native.Globals.MainChecker = checker
		native.Globals.Unlock()
	}
}
