package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestActualSMTMemoryWatermarkUnknown(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	compiled := compileSource(t, "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }")
	executor := New()
	ctx := context.Background()
	options := CheckOptions{MaxSteps: 1}
	baseline := func() {
		t.Helper()
		result, err := executor.Check(ctx, compiled, nil, options)
		if err != nil || result.Outcome != CheckSafe || len(result.Counterexamples) != 0 {
			t.Fatalf("unrestricted checker failed: %+v, %v", result, err)
		}
		assertNoCommandTemporarySources(t, directory)
	}
	baseline()
	// This supported solver watermark deterministically returns SMTUnknown on
	// the real query. It is a solver setting, not a process-wide memory cap, and
	// the caller context remains active. No service or solver is substituted.
	loaded := json.RawMessage(`{"checker":{"tuning":{"z3.memory_high_watermark_mb":"1"}}}`)
	result, err := executor.Verify(ctx, compiled, loaded, VerificationOptions{CheckOptions: options})
	var phaseError *VerificationError
	if !errors.As(err, &phaseError) || phaseError.Phase != BoundedPhase || len(result.Phases) != 1 || result.Outcome != "" || len(result.Counterexamples) != 0 || len(result.Data) != 0 || ctx.Err() != nil {
		t.Fatalf("solver unknown became a verdict: %+v, %v", result, err)
	}
	failure := assertActualIncompleteOutcome(t, err, "Unknown")
	report := ReportVerification(result, err, nil)
	if report.Explanation != "verification was inconclusive (Unknown)" || report.Phase != BoundedPhase || report.BackendData != failure.Data || len(report.Traces) != 0 || len(report.Errors) != 0 {
		t.Fatalf("inconclusive solver report lost information: %+v", report)
	}
	assertNoCommandTemporarySources(t, directory)
	// A subsequent unrestricted query through the same service must recover;
	// the resource setting belongs only to its own checker invocation.
	baseline()
}

func TestActualEmptyExecutionsRemainInconclusive(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	for _, tc := range []struct{ name, init, tuning string }{
		{"unsatisfiable initializer", "all { x'=0, false }", ""},
		{"all initial transitions filtered", "x'=0", `,"tuning":{"search.transitionFilter":"1->0"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled := compileSource(t, "module M { var x:int action init="+tc.init+" action step=x'=x val safe=x>=0 }")
			for _, noDeadlock := range []bool{true, false} {
				t.Run(fmt.Sprintf("deadlock checking %v", !noDeadlock), func(t *testing.T) {
					loaded := json.RawMessage(fmt.Sprintf(`{"checker":{"no-deadlock":%v%s}}`, noDeadlock, tc.tuning))
					result, err := New().Check(context.Background(), compiled, loaded, CheckOptions{MaxSteps: 1})
					if len(result.Counterexamples) != 0 {
						t.Fatal("no initial execution exists to provide a counterexample", result)
					}
					if noDeadlock {
						if result.Outcome != "" || len(result.Data) != 0 {
							t.Fatalf("empty execution became a verdict: %+v, %v", result, err)
						}
						failure := assertActualIncompleteOutcome(t, err, "ExecutionsTooShort")
						report := ReportCheck(result, err, nil)
						if report.Explanation != "verification was inconclusive (ExecutionsTooShort)" || report.BackendData != failure.Data || len(report.Traces) != 0 {
							t.Fatal(report)
						}
					} else {
						if err != nil || result.Outcome != CheckDeadlock {
							t.Fatalf("deadlock outcome changed: %+v, %v", result, err)
						}
						report := ReportCheck(result, nil, nil)
						if report.Explanation != "reached a deadlock" || len(report.Traces) != 0 {
							t.Fatal(report)
						}
					}
					assertNoCommandTemporarySources(t, directory)
				})
			}
		})
	}
}

func assertActualIncompleteOutcome(t *testing.T, err error, status string) *CommandFailure {
	t.Helper()
	var failure *CommandFailure
	if !errors.As(err, &failure) || failure.Type != "PASS_FAILURE" {
		t.Fatalf("expected a checker-pass failure, got %T %v", err, err)
	}
	var payload struct {
		Pass     string `json:"pass_name"`
		ExitCode int    `json:"exit_code"`
		Error    struct {
			Status          string            `json:"checking_result"`
			Counterexamples []json.RawMessage `json:"counterexamples"`
		} `json:"error_data"`
	}
	if err := json.Unmarshal([]byte(failure.Data), &payload); err != nil || payload.Pass != "BoundedChecker" || payload.ExitCode != 12 || payload.Error.Status != status || len(payload.Error.Counterexamples) != 0 {
		t.Fatalf("unexpected actual checker payload: %s, %v", failure.Data, err)
	}
	return failure
}
