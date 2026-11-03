package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	pal "github.com/glycerine/apalache-go"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

// Cancellation starts at the real service's first context-value lookup, after
// Run's entry check and source materialization. This avoids wall-clock races and
// does not replace the command service or solver with a test implementation.
type serviceCancellationContext struct {
	context.Context
	once   sync.Once
	cancel context.CancelFunc
	called bool
}

func (ctx *serviceCancellationContext) Value(key any) any {
	ctx.once.Do(func() {
		ctx.called = true
		ctx.cancel()
	})
	return ctx.Context.Value(key)
}

func TestActualServiceCancellationAndSMTTimeout(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	compiled := compileSource(t, "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }")
	for _, verify := range []bool{false, true} {
		name := "check"
		if verify {
			name = "verify"
		}
		t.Run(name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &serviceCancellationContext{Context: parent, cancel: cancel}
			if verify {
				result, err := New().Verify(ctx, compiled, nil, VerificationOptions{CheckOptions: CheckOptions{MaxSteps: 1}})
				var phaseError *VerificationError
				if !ctx.called || !errors.Is(err, context.Canceled) || !errors.As(err, &phaseError) || phaseError.Phase != BoundedPhase || result.Outcome != "" || len(result.Phases) != 1 {
					t.Fatalf("cancellation became a checker result: %+v, %v (service entered %v)", result, err, ctx.called)
				}
			} else {
				result, err := New().Check(ctx, compiled, nil, CheckOptions{MaxSteps: 1})
				if !ctx.called || !errors.Is(err, context.Canceled) || result.Outcome != "" || len(result.Counterexamples) != 0 {
					t.Fatalf("cancellation became a checker result: %+v, %v (service entered %v)", result, err, ctx.called)
				}
			}
			assertNoCommandTemporarySources(t, directory)
		})
	}
	t.Run("SMT-query-deadline", func(t *testing.T) {
		// A per-query budget is distinct from cancellation of the caller. The
		// real solver must fail explicitly while the parent remains active.
		ctx := pal.WithSMTQueryTimeout(context.Background(), time.Nanosecond)
		result, err := New().Check(ctx, compiled, nil, CheckOptions{MaxSteps: 1})
		var failure *CommandFailure
		if !errors.As(err, &failure) || failure.Type != "UNEXPECTED" || !strings.Contains(failure.Data, context.DeadlineExceeded.Error()) || ctx.Err() != nil || result.Outcome != "" || len(result.Counterexamples) != 0 {
			t.Fatalf("SMT exhaustion became a checker result: %+v, %v", result, err)
		}
		report := ReportCheck(result, err, nil)
		if report.Explanation != context.DeadlineExceeded.Error() || report.BackendData != failure.Data || len(report.Traces) != 0 {
			t.Fatalf("SMT exhaustion report lost the backend failure: %+v", report)
		}
		assertNoCommandTemporarySources(t, directory)
	})
}

func TestActualCommandTemporarySourceCleanup(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	t.Run("successful-TLA", func(t *testing.T) {
		if _, err := New().TLA(context.Background(), compiled); err != nil {
			t.Fatal(err)
		}
		assertNoCommandTemporarySources(t, directory)
	})
	t.Run("checker-validation-failure", func(t *testing.T) {
		result, err := New().Check(context.Background(), compiled, json.RawMessage(`{"checker":{"smt-solver":"unsupported-solver"}}`), CheckOptions{})
		var failure *CommandFailure
		if !errors.As(err, &failure) || failure.Type != "UNEXPECTED" || !strings.Contains(failure.Data, "unsupported SMT solver") || result.Outcome != "" {
			t.Fatal(result, err)
		}
		assertNoCommandTemporarySources(t, directory)
	})
	t.Run("malformed-IR", func(t *testing.T) {
		config := json.RawMessage(`{"source":{"kind":"string","format":"qnt","content":"{not valid IR}"}}`)
		response, err := New().Run(context.Background(), cmd.Cmd_CHECK, config)
		if err != nil {
			t.Fatal(err)
		}
		result, err := decodeCheckResponse(response)
		var failure *CommandFailure
		if !errors.As(err, &failure) || failure.Type != "PASS_FAILURE" || !strings.Contains(failure.Data, "SanyParser") || result.Outcome != "" {
			t.Fatal(result, err)
		}
		assertNoCommandTemporarySources(t, directory)
	})
}

func TestPassFailurePayloadsRemainInspectable(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"pass_name":"SanyParser","error_data":"bad input"}`,
		`{"pass_name":"TypeCheckerSnowcat","error_data":[["model.qnt:1","bad type"]]}`,
		`{"pass_name":"FuturePass","error_data":{"large":9007199254740993}}`,
		`{"pass_name":"BoundedChecker","error_data":"bad checker payload"}`,
		`{`,
	} {
		response := &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{ErrorType: cmd.CmdErrorType_PASS_FAILURE, Data: payload}}}
		result, err := decodeCheckResponse(response)
		var failure *CommandFailure
		if !errors.As(err, &failure) || failure.Type != "PASS_FAILURE" || failure.Data != payload || result.Outcome != "" || len(result.Counterexamples) != 0 {
			t.Fatalf("backend failure payload was lost: %+v, %v", result, err)
		}
		report := ReportCheck(result, err, nil)
		if report.BackendData != payload || report.Explanation == "" || len(report.Traces) != 0 {
			t.Fatalf("backend failure report was lost: %+v", report)
		}
	}
}

func assertNoCommandTemporarySources(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("command service left temporary sources: %v, %v", entries, err)
	}
}
