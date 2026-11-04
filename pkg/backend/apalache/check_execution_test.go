package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pal "github.com/glycerine/apalache-go"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

func TestCheckExecutionPreservesServiceValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, config, message string
		transport             bool
	}{
		{"unknown top level", `{"unknown":true}`, "unknown field", true},
		{"unknown checker option", `{"checker":{"unknown":true}}`, "unknown field", true},
		{"file source", `{"source":"secret.tla"}`, "File-backed sources", true},
		{"file output", `{"output":"result.json"}`, "Filesystem paths", true},
		{"unknown feature", `{"features":["unrecognized"]}`, "Unexpected feature", true},
		{"invalid typed scalar", `{"checker":{"length":"zero"}}`, "cannot unmarshal", true},
		{"unknown source kind", `{"source":{"kind":"network","content":""}}`, "expected string", true},
		{"missing source before checker", `{"checker":{"length":-1}}`, "Missing value for required option source", false},
		{"source materialization before checker", `{"source":{"kind":"string","format":"unknown","content":""},"checker":{"length":-1}}`, "unsupported source format", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := New()
			// Invalid requests never enter the solver. The service remains the
			// independent reference for its exact schema and source policy.
			reference, referenceError := executor.service.Run(context.Background(), &cmd.CmdRequest{Cmd: cmd.Cmd_CHECK, Config: tc.config})
			response, err := executor.Run(context.Background(), cmd.Cmd_CHECK, json.RawMessage(tc.config))
			if tc.transport {
				if err == nil || referenceError == nil || !strings.Contains(err.Error(), tc.message) || err.Error() != referenceError.Error() || response != nil {
					t.Fatalf("adapter=%+v,%v service=%+v,%v", response, err, reference, referenceError)
				}
				return
			}
			if err != nil || referenceError != nil || response.GetFailure() == nil || reference.GetFailure() == nil || response.GetFailure().GetErrorType() != reference.GetFailure().GetErrorType() || response.GetFailure().GetData() != reference.GetFailure().GetData() || !strings.Contains(response.GetFailure().GetData(), tc.message) {
				t.Fatalf("adapter=%+v,%v service=%+v,%v", response, err, reference, referenceError)
			}
		})
	}
}

func TestCheckValidationPrecedesFrontendFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ option, message string }{
		{`"length":-1`, "checker.length cannot be negative"},
		{`"max-error":0`, "checker.max-error must be positive"},
		{`"max-error":2`, "requires checker.view"},
		{`"timeout-smt":-1`, "checker.timeout-smt cannot be negative"},
		{`"timeout-smt":9223372036854775807`, "checker.timeout-smt is too large"},
		{`"init":"OnlyInit"`, "checker.init and checker.next must be provided together"},
		{`"algo":"offline"`, "unsupported checker algorithm"},
		{`"smt-solver":"other"`, "unsupported SMT solver"},
		{`"smt-encoding":"other"`, "Unexpected SMT encoding"},
		{`"tuning":{"search.invariantMode":"other"}`, "invalid checker tuning"},
	} {
		t.Run(tc.option, func(t *testing.T) {
			config := json.RawMessage(`{"source":{"kind":"string","format":"qnt","content":"malformed IR"},"checker":{` + tc.option + `}}`)
			response, err := New().Run(context.Background(), cmd.Cmd_CHECK, config)
			if err != nil || response.GetFailure() == nil || response.GetFailure().GetErrorType() != cmd.CmdErrorType_UNEXPECTED || !strings.Contains(response.GetFailure().GetData(), tc.message) {
				t.Fatalf("checker validation was displaced by frontend failure: %+v, %v", response, err)
			}
		})
	}
}

func TestCheckExecutionSupportsRawTLAAndCanonicalIR(t *testing.T) {
	t.Parallel()
	const source = `---- MODULE M ----
EXTENDS Integers, Helper
\* @type: Int;
VARIABLE x
Start == x = Seed
Advance == x' = 1-x
Good == x \in {0,1}
Bad == x = 0
====`
	const helper = "---- MODULE Helper ----\nSeed == 0\n===="
	executor := New()
	configuration := func(format, content string, aux []string, invariant string) json.RawMessage {
		t.Helper()
		encoded, err := json.Marshal(map[string]any{
			"source":  map[string]any{"kind": "string", "format": format, "content": content, "aux": aux},
			"checker": map[string]any{"init": "Start", "next": "Advance", "inv": []string{invariant}, "length": 1},
		})
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	typed, err := executor.Run(context.Background(), cmd.Cmd_TYPECHECK, configuration("tla", source, []string{helper}, "Good"))
	if err != nil || typed.GetSuccess() == "" {
		t.Fatal(typed, err)
	}
	for _, format := range []string{"tla", "json"} {
		for _, invariant := range []string{"Good", "Bad"} {
			t.Run(format+"/"+invariant, func(t *testing.T) {
				content, aux := source, []string{helper}
				if format == "json" {
					content, aux = typed.GetSuccess(), nil
				}
				response, err := executor.Run(context.Background(), cmd.Cmd_CHECK, configuration(format, content, aux, invariant))
				if err != nil {
					t.Fatal(err)
				}
				result, err := decodeCheckResponse(response)
				if err != nil {
					t.Fatal(err)
				}
				if invariant == "Good" {
					if result.Outcome != CheckSafe || len(result.Counterexamples) != 0 {
						t.Fatal(result)
					}
					return
				}
				if result.Outcome != CheckViolation || len(result.Counterexamples) != 1 || !strings.Contains(string(result.Counterexamples[0]), `"#bigint":"0"`) || !strings.Contains(string(result.Counterexamples[0]), `"#bigint":"1"`) {
					t.Fatal("custom operators or imported constant changed", result)
				}
			})
		}
	}
}

func TestUnknownCheckerOutcomeCannotBecomeSafe(t *testing.T) {
	t.Parallel()
	for _, outcome := range []pal.SafetyOutcome{pal.SafetyUnknown, pal.SafetyExecutionsTooShort, pal.SafetyOutcome(255)} {
		response, err := encodeBoundedCheckResponse(nil, pal.BoundedSafetyResult{Outcome: outcome})
		if err != nil {
			t.Fatal(err)
		}
		result, err := decodeCheckResponse(response)
		var failure *CommandFailure
		if !errors.As(err, &failure) || result.Outcome != "" || len(result.Counterexamples) != 0 || len(result.Data) != 0 || response.GetSuccess() != "" {
			t.Fatalf("incomplete or unrecognized checker outcome became a verdict: %+v, %v", result, err)
		}
	}
}
