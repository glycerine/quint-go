package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestCheckerFailureReports(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure CommandFailure
		want    string
	}{
		{"message", CommandFailure{Type: "UNEXPECTED", Data: `{"msg":"unsupported operator"}`}, "unsupported operator"},
		{"assignment", CommandFailure{Type: "UNEXPECTED", Data: `{"msg":"Assignment error: <[UNKNOWN]>: A::B::x' is used before it is assigned"}`}, "x is used before it is assigned. You need to have either `x == <expr>` or `x.in(<set>)` before doing anything else with `x` in your predicate."},
		{"go-assignment", CommandFailure{Type: "UNEXPECTED", Data: `{"msg":"extracting q::inductiveInvPrimed: <[UNKNOWN]>: x' is used before it is assigned. See docs"}`}, "x is used before it is assigned. You need to have either `x == <expr>` or `x.in(<set>)` before doing anything else with `x` in your predicate."},
		{"parser", CommandFailure{Type: "PASS_FAILURE", Data: `{"pass_name":"SanyParser","error_data":"bad syntax"}`}, "internal error: while parsing in Apalache:\n'bad syntax'\nPlease report an issue: https://github.com/informalsystems/quint/issues/new"},
		{"types", CommandFailure{Type: "PASS_FAILURE", Data: `{"pass_name":"TypeCheckerSnowcat","error_data":"bad type"}`}, "internal error: while type checking in Apalache:\n'bad type'\nPlease report an issue: https://github.com/informalsystems/quint/issues/new"},
		{"unknown", CommandFailure{Type: "PASS_FAILURE", Data: `{"pass_name":"BoundedChecker","error_data":{"checking_result":"Unknown"}}`}, "verification was inconclusive (Unknown)"},
		{"short", CommandFailure{Type: "PASS_FAILURE", Data: `{"pass_name":"BoundedChecker","error_data":{"checking_result":"ExecutionsTooShort"}}`}, "verification was inconclusive (ExecutionsTooShort)"},
		{"future", CommandFailure{Type: "FUTURE", Data: `{"x":1}`}, `FUTURE: {"x":1}`},
		{"malformed", CommandFailure{Type: "PASS_FAILURE", Data: `{`}, "PASS_FAILURE: {"},
		{"bad-message", CommandFailure{Type: "UNEXPECTED", Data: `{"msg":42}`}, `UNEXPECTED: {"msg":42}`},
		{"no-message", CommandFailure{Type: "UNEXPECTED", Data: `{}`}, `UNEXPECTED: {}`},
		{"other-pass", CommandFailure{Type: "PASS_FAILURE", Data: `{"pass_name":"Other","error_data":{}}`}, `PASS_FAILURE: {"pass_name":"Other","error_data":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := ReportVerification(VerificationResult{}, &VerificationError{Phase: PreservationPhase, Err: &tc.failure}, nil)
			encoded, err := json.Marshal(report)
			if err != nil {
				t.Fatal("report cannot be serialized", err)
			}
			if !strings.Contains(string(encoded), `"explanation":`) || !strings.Contains(string(encoded), `"errors":[]`) {
				t.Fatal("report schema", string(encoded))
			}
			if report.Explanation != tc.want || report.Phase != PreservationPhase || len(report.Errors) != 0 || string(report.BackendData) != tc.failure.Data {
				t.Fatalf("%+v", report)
			}
		})
	}
}

func TestCheckerOutcomeReports(t *testing.T) {
	trace := json.RawMessage(`{"states":[{"x":9007199254740993}],"#meta":{"loop":0}}`)
	for _, tc := range []struct {
		outcome     CheckOutcome
		explanation string
	}{{CheckSafe, ""}, {CheckViolation, "found a counterexample"}, {CheckDeadlock, "reached a deadlock"}} {
		data := json.RawMessage(`{"raw":true}`)
		report := ReportCheck(CheckResult{Outcome: tc.outcome, Counterexamples: []json.RawMessage{trace}, Data: data}, nil, nil)
		if report.Explanation != tc.explanation {
			t.Fatal(report)
		}
		data[0] = 'x'
		if report.BackendData != `{"raw":true}` {
			t.Fatal("backend data aliased")
		}
		if tc.outcome != CheckSafe {
			if len(report.Traces) != 1 || !strings.Contains(string(report.Traces[0]), "9007199254740993") || !strings.Contains(string(report.Traces[0]), `"loop":0`) {
				t.Fatal(report)
			}
			report.Traces[0][0] = 'x'
			if trace[0] != '{' {
				t.Fatal("trace aliased")
			}
		}
	}
	for _, status := range []string{"Error", "Deadlock"} {
		payload := `{"pass_name":"BoundedChecker","error_data":{"checking_result":"` + status + `","counterexamples":[` + string(trace) + `]}}`
		report := ReportCheck(CheckResult{}, &CommandFailure{Type: "PASS_FAILURE", Data: payload}, nil)
		if len(report.Traces) != 1 || strings.Contains(report.Explanation, "inconclusive") {
			t.Fatal(report)
		}
	}
	if report := ReportCheck(CheckResult{}, nil, nil); !strings.Contains(report.Explanation, "invalid checker outcome") {
		t.Fatal(report)
	}
	if report := ReportCheck(CheckResult{}, context.Canceled, nil); report.Explanation != "context canceled" {
		t.Fatal(report)
	}
}

func TestPreparationReportLocations(t *testing.T) {
	id, _ := quint.ParseID("5")
	location := quint.Location{Source: "model.qnt"}
	err := &PreparationError{Diagnostics: []quint.Diagnostic{{Code: quint.QNT409, Message: "shared init", Reference: &id}}}
	report := ReportVerification(VerificationResult{Phases: []VerificationPhaseResult{{Phase: InitializationPhase}}}, err, quint.SourceMap{id: location})
	if report.Explanation != "Failed to convert init to predicate" || len(report.Errors) != 1 || len(report.Errors[0].Locations) != 1 || report.Errors[0].Locations[0].Source != "model.qnt" || report.Phase != InitializationPhase {
		t.Fatal(report)
	}
	if !strings.Contains(report.Errors[0].Explanation, "QNT409") {
		t.Fatal(report)
	}
	plain := errors.New("network unavailable")
	if got := ReportCheck(CheckResult{}, plain, nil); got.Explanation != plain.Error() {
		t.Fatal(got)
	}
}
