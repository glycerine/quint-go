package quint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type replCancellationWriter struct {
	cancel context.CancelFunc
	err    error
	writes int
}

func (writer *replCancellationWriter) Write(data []byte) (int, error) {
	writer.writes++
	if writer.cancel != nil {
		writer.cancel()
	}
	if writer.err != nil {
		return 0, writer.err
	}
	return len(data), nil
}

func TestREPLCancellationAfterFinalRuntimeCallback(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		name, input, state string
		initialize         bool
	}{
		{"new-variable", `all { x'=1, q::debug("cancel", true) }`, `Rec()`, false},
		{"initialized", `all { x'=1, q::debug("cancel", true) }`, `Rec("x", 0)`, true},
		{"false-result", `all { x'=1, q::debug("cancel", false) }`, `Rec("x", 0)`, true},
		{"nested-cancellation", `all { x'=1, q::debug("cancel", true), true }`, `Rec("x", 0)`, true},
		// then commits its first action inside the evaluator. Cancelling the
		// final action prevents the REPL commit without rewinding that shift.
		{"internal-shift", `(x'=1).then(all { x'=2, q::debug("cancel", true) })`, `Rec("x", 1)`, true},
	} {
		t.Run(example.name, func(t *testing.T) {
			session := NewREPLSession()
			if _, err := session.Evaluate(context.Background(), "var x:int"); err != nil {
				t.Fatal(err)
			}
			if example.initialize {
				replEvaluate(t, session, "x'=0", "true")
			}
			before := len(session.History())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &replCancellationWriter{cancel: cancel}
			ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: writer})
			result, err := session.Evaluate(ctx, example.input)
			if !errors.Is(err, context.Canceled) || result.Kind != "value" || result.Value.Valid() || result.Shifted || result.OldState.Valid() || result.NewState.Valid() || writer.writes != 1 {
				t.Fatal("cancelled action returned a successful commit", result, err, writer.writes)
			}
			state, err := session.State()
			if err != nil || state.String() != example.state {
				t.Fatal("cancelled action changed current state", state.String(), err)
			}
			if history := session.History(); len(history) != before+1 || history[len(history)-1] != example.input {
				t.Fatal("accepted cancelled expression lost its history", history)
			}
			// A later true input must not commit the cancelled assignments.
			if next := replEvaluate(t, session, "true", "true"); next.Shifted {
				t.Fatal("cancelled action leaked next-state assignments", next)
			}
			replEvaluate(t, session, "x'=3", "true")
			replEvaluate(t, session, "x", "3")
		})
	}
}

func TestREPLCancellationPreservesPreviousPendingState(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	if _, err := session.Evaluate(context.Background(), "var x:int"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "x'=0", "true")
	writeErr := errors.New("debug output stopped")
	writer := &replCancellationWriter{err: writeErr}
	ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: writer})
	if result, err := session.Evaluate(ctx, `all { x'=9, q::debug("error", true) }`); !errors.Is(err, writeErr) || result.Shifted {
		t.Fatal("ordinary runtime error contract changed", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: &replCancellationWriter{cancel: cancel}})
	if _, err := session.Evaluate(ctx, `all { x'=1, q::debug("cancel", true) }`); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	result := replEvaluate(t, session, "true", "true")
	if !result.Shifted || result.OldState.String() != `Rec("x", 0)` || result.NewState.String() != `Rec("x", 9)` {
		t.Fatal("cancellation did not restore the previous pending state", result)
	}
}

func TestREPLReplayReportsFinalCallbackCancellationBeforeCommit(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.qnt")
	text := "// @mainModule M\nmodule M { var x:int }\n/*! x'=0 !*/\n/*! all { x'=1, q::debug(\"cancel\", true) } !*/\n/*! x'=2 !*/\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: &replCancellationWriter{cancel: cancel}})
	var results []REPLReplayResult
	err := session.LoadWithReplay(ctx, path, "", REPLReplayHooks{After: func(result REPLReplayResult) error {
		results = append(results, result)
		return nil
	}})
	if !errors.Is(err, context.Canceled) || len(results) != 2 || results[0].Err != nil || !results[0].Result.Shifted || !errors.Is(results[1].Err, context.Canceled) || results[1].Result.Shifted || results[1].Result.Value.Valid() {
		t.Fatal("replay reported a successful cancelled action", err, results)
	}
	state, err := session.State()
	if err != nil || state.String() != `Rec("x", 0)` || len(session.History()) != 2 {
		t.Fatal("replay committed or continued after cancellation", state.String(), err, session.History())
	}
	if result := replEvaluate(t, session, "true", "true"); result.Shifted {
		t.Fatal("cancelled replay leaked pending assignments", result)
	}
}
