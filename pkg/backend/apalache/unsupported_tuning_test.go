package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

func TestIneffectiveCheckControlsFailBeforeExecution(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	for _, test := range []struct{ key, value string }{
		{"search.simulation", "true"},
		{"search.simulation.maxRun", "1"},
		{"search.split.timeout.minimum", "1"},
		{"search.split.timeout.factor", "2"},
		{"search.idle.timeout", "1"},
		{"z3.rlimit", "1"},
		{"z3.reproducible_resource_limit", "2"},
		{"z3.reproducible-resource-limit", "2"},
		{"z3.smt.rlimit", "2"},
		{"z3.sat.cardinality.encoding", "grouped"},
	} {
		t.Run(test.key, func(t *testing.T) {
			config := json.RawMessage(fmt.Sprintf(`{"checker":{"tuning":{%q:%q}}}`, test.key, test.value))
			// A raw command cannot bypass the same guard used by Check/Verify.
			// No source is needed: unsupported controls must fail before the
			// command service can materialize input or initialize a solver.
			response, err := New().Run(context.Background(), cmd.Cmd_CHECK, config)
			var unsupported *UnsupportedCheckOptionError
			if response != nil || !errors.As(err, &unsupported) || unsupported.Option != "checker.tuning."+test.key {
				t.Fatal(response, err)
			}
			report := ReportCheck(CheckResult{}, err, nil)
			if !strings.Contains(report.Explanation, test.key) || report.BackendData != "" || len(report.Traces) != 0 {
				t.Fatal(report)
			}
			assertNoCommandTemporarySources(t, directory)
		})
	}
}

func TestEffectiveCheckTuningUsesNativeJSONMergeSemantics(t *testing.T) {
	t.Parallel()
	for _, config := range []string{
		`{"Checker":{"tuning":{"search.simulation":"true"}}}`,
		`{"checker":{"Tuning":{"search.simulation":"true"}}}`,
		`{"CHECKER":{"TUNING":{"search.simulation":"true"}}}`,
		`{"checker":{"tuning":{"search.simulation":"true"}},"checker":{}}`,
		`{"checker":{"tuning":{"search.simulation":"true"},"tuning":{}}}`,
		`{"checker":{"tuning":{"search.simulation":"true"}},"CHECKER":null}`,
	} {
		response, err := New().Run(context.Background(), cmd.Cmd_CHECK, json.RawMessage(config))
		var unsupported *UnsupportedCheckOptionError
		if response != nil || !errors.As(err, &unsupported) || unsupported.Option != "checker.tuning.search.simulation" {
			t.Fatal("effective ignored control bypassed validation", config, response, err)
		}
	}
	for _, config := range []string{
		`{"checker":{"tuning":{"search.simulation":"true"},"TUNING":{"search.simulation":"false"}}}`,
		`{"checker":{"tuning":{"search.simulation":"true"}},"CHECKER":{"Tuning":null}}`,
	} {
		if err := validateCheckTuning(json.RawMessage(config)); err != nil {
			t.Fatal("effective disabled control was rejected", config, err)
		}
	}
}

func TestEffectiveConfigurationPrecedenceAndSupportedControls(t *testing.T) {
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	loaded := json.RawMessage(`{"checker":{"timeout-smt":1,"tuning":{"search.simulation":"true","z3.rlimit":"0","z3.timeout":"0","z3.memory_high_watermark_mb":"0","search.invariant.mode":"after","rewriter.shortCircuit":"true"}}}`)
	// Explicit false overrides the loaded simulation setting before validation.
	result, err := New().Check(context.Background(), compiled, loaded, CheckOptions{MaxSteps: 0})
	if err != nil || result.Outcome != CheckSafe {
		t.Fatal("supported controls or effective precedence changed", result, err)
	}
	for _, verify := range []bool{false, true} {
		var unsupported *UnsupportedCheckOptionError
		if verify {
			result, err := New().Verify(context.Background(), compiled, nil, VerificationOptions{CheckOptions: CheckOptions{RandomTransitions: true}})
			var phaseError *VerificationError
			if !errors.As(err, &unsupported) || !errors.As(err, &phaseError) || phaseError.Phase != BoundedPhase || result.Outcome != "" || len(result.Phases) != 1 {
				t.Fatal("random transitions became a checker verdict", result, err)
			}
		} else {
			result, err := New().Check(context.Background(), compiled, nil, CheckOptions{RandomTransitions: true})
			if !errors.As(err, &unsupported) || result.Outcome != "" {
				t.Fatal("random transitions became a checker verdict", result, err)
			}
		}
		if unsupported.Option != "checker.tuning.search.simulation" {
			t.Fatal(unsupported)
		}
	}
	// Config construction remains available for upstream-compatible inspection;
	// unsupported behavior is rejected only when a CHECK is executed.
	config, err := CheckConfig(compiled, nil, CheckOptions{RandomTransitions: true})
	if err != nil || !strings.Contains(string(config), `"search.simulation":"true"`) {
		t.Fatal(string(config), err)
	}
}

func TestUnsupportedControlValidationLeavesStrictParsingAuthoritative(t *testing.T) {
	t.Parallel()
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	input, err := MarshalInput(compiled)
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{
		`[]`, `{"checker":1}`, `{"checker":{"tuning":[]}}`,
		`{"checker":{"tuning":{"search.simulation":true}}}`,
		`{"checker":{"tuning":{"search.simulation":"TRUE"}}}`,
		`{"checker":{"tuning":{"z3.rlimit":"invalid"}}}`,
		`{"checker":{"tuning":{"search.idle.timeout":null}}}`,
		`{"checker":{"tuning":{"search.simulation.maxRun":"abc"}}}`,
		`{"checker":{"tuning":{"search.simulation.maxRun":"0"}}}`,
		`{"checker":{"tuning":{"search.split.timeout.minimum":"abc"}}}`,
		`{"checker":{"tuning":{"search.split.timeout.factor":"2147483648"}}}`,
		`{"checker":{"tuning":{"search.idle.timeout":"abc"}}}`,
		`{"checker":{"tuning":{"z3.sat.cardinality.encoding":"invalid"}}}`,
		`{"checker":{"tuning":{"z3.rlimit":"2147483648"}}}`,
		`{"checker":{"tuning":{"z3.reproducible_resource_limit":"1"}}}`,
		`{"checker":{"tuning":{"z3.reproducible-resource-limit":"invalid"}}}`,
		`{"checker":{"tuning":{"z3.smt.rlimit":"1"}}}`,
	} {
		if err := validateCheckTuning(json.RawMessage(config)); err != nil {
			t.Fatal("adapter replaced strict malformed-config validation", config, err)
		}
		if object, err := readConfigObject(json.RawMessage(config)); err == nil {
			if err := object.set("source", quintSource{Kind: "string", Format: "qnt", Content: string(input)}); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			config = string(encoded)
		}
		response, err := New().Run(context.Background(), cmd.Cmd_CHECK, json.RawMessage(config))
		if err == nil {
			_, err = decodeCheckResponse(response)
		}
		var unsupported *UnsupportedCheckOptionError
		if err == nil || errors.As(err, &unsupported) {
			t.Fatal("strict service validation was bypassed", config, response, err)
		}
	}
}
