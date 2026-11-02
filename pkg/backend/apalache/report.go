package apalache

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	quint "github.com/glycerine/quint-go/pkg"
)

// VerificationReport is independent of terminal rendering and trace files.
// BackendData retains raw backend text, including malformed JSON; Traces retain
// lossless JSON for subsequent ITF processing.
type VerificationReport struct {
	Explanation string               `json:"explanation"`
	Errors      []quint.ErrorMessage `json:"errors"`
	Traces      []json.RawMessage    `json:"traces,omitempty"`
	BackendData string               `json:"backendData,omitempty"`
	Phase       VerificationPhase    `json:"phase,omitempty"`
}

var unassignedVariable = regexp.MustCompile(`<\[UNKNOWN\]>: (?:\w+::)*(\w+)' is used before it is assigned`)

// ReportVerification adds the proof phase to the checker report. It also works
// for errors returned between phases, when no checker result exists yet.
func ReportVerification(result VerificationResult, err error, sources quint.SourceMap) VerificationReport {
	report := ReportCheck(result.CheckResult, err, sources)
	if len(result.Phases) > 0 {
		report.Phase = result.Phases[len(result.Phases)-1].Phase
	}
	var phaseError *VerificationError
	if errors.As(err, &phaseError) {
		report.Phase = phaseError.Phase
	}
	return report
}

// ReportCheck maps checker outcomes and backend errors to Quint's messages.
// Unknown passes/results and malformed error data remain visible, rather than
// panicking or implying success. Source diagnostics are resolved when available.
func ReportCheck(result CheckResult, err error, sources quint.SourceMap) VerificationReport {
	report := VerificationReport{Errors: []quint.ErrorMessage{}}
	if err == nil {
		report.BackendData = string(result.Data)
		switch result.Outcome {
		case CheckSafe:
			return report
		case CheckViolation:
			report.Explanation = "found a counterexample"
		case CheckDeadlock:
			report.Explanation = "reached a deadlock"
		default:
			report.Explanation = fmt.Sprintf("invalid checker outcome %q", result.Outcome)
			return report
		}
		for _, trace := range result.Counterexamples {
			report.Traces = append(report.Traces, append(json.RawMessage(nil), trace...))
		}
		return report
	}
	report.Explanation = err.Error()
	var preparation *PreparationError
	if errors.As(err, &preparation) {
		report.Explanation = "Failed to convert init to predicate"
		for _, d := range preparation.Diagnostics {
			report.Errors = append(report.Errors, quint.ResolveDiagnostic(sources, d))
		}
		return report
	}
	var failure *CommandFailure
	if !errors.As(err, &failure) {
		return report
	}
	report.BackendData = failure.Data
	report.Explanation = failure.Error()
	if failure.Type == "UNEXPECTED" {
		var data struct {
			Message *string `json:"msg"`
		}
		if json.Unmarshal([]byte(failure.Data), &data) == nil && data.Message != nil {
			report.Explanation = *data.Message
			if match := unassignedVariable.FindStringSubmatch(*data.Message); match != nil {
				name := match[1]
				report.Explanation = fmt.Sprintf("%s is used before it is assigned. You need to have either `%s == <expr>` or `%s.in(<set>)` before doing anything else with `%s` in your predicate.", name, name, name, name)
			}
		}
		return report
	}
	if failure.Type != "PASS_FAILURE" {
		return report
	}
	var data struct {
		Pass  string          `json:"pass_name"`
		Error json.RawMessage `json:"error_data"`
	}
	if json.Unmarshal([]byte(failure.Data), &data) != nil {
		return report
	}
	switch data.Pass {
	case "SanyParser", "TypeCheckerSnowcat":
		detail := string(data.Error)
		var text string
		if json.Unmarshal(data.Error, &text) == nil {
			detail = text
		}
		operation := "parsing"
		if data.Pass == "TypeCheckerSnowcat" {
			operation = "type checking"
		}
		report.Explanation = fmt.Sprintf("internal error: while %s in Apalache:\n'%s'\nPlease report an issue: https://github.com/informalsystems/quint/issues/new", operation, detail)
	case "BoundedChecker":
		var status struct {
			Result          string            `json:"checking_result"`
			Counterexamples []json.RawMessage `json:"counterexamples"`
		}
		if json.Unmarshal(data.Error, &status) == nil && status.Result != "" {
			switch status.Result {
			case "Error":
				report.Explanation = "found a counterexample"
			case "Deadlock":
				report.Explanation = "reached a deadlock"
			default:
				report.Explanation = "verification was inconclusive (" + status.Result + ")"
				return report
			}
			for _, trace := range status.Counterexamples {
				report.Traces = append(report.Traces, append(json.RawMessage(nil), trace...))
			}
		}
	}
	return report
}
