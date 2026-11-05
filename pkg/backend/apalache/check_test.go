package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	cmd "github.com/glycerine/apalache-go/pkg/shai/cmdexecutor"
	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamCheckConfiguration(t *testing.T) {
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	var fixtures []struct {
		Name           string
		Loaded, Output json.RawMessage
		Options        CheckOptions
	}
	data, err := os.ReadFile("../../testdata/parser/check_config.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			before := string(f.Loaded)
			config, err := CheckConfig(compiled, f.Loaded, f.Options)
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			json.Unmarshal(config, &got)
			json.Unmarshal(f.Output, &want)
			got["source"].(map[string]any)["content"] = "<IR>"
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Go %s\nTS %s", config, f.Output)
			}
			if string(f.Loaded) != before {
				t.Fatal("loaded config changed")
			}
		})
	}
	for _, bad := range []string{`{`, `[]`, `{} {}`, `{"checker":1}`, `{"checker":{"tuning":[]}}`} {
		if _, err := CheckConfig(compiled, json.RawMessage(bad), CheckOptions{}); err == nil {
			t.Fatal("malformed config accepted", bad)
		}
	}
	if _, err := CheckConfig(compiled, nil, CheckOptions{MaxSteps: -1}); err == nil {
		t.Fatal("negative bound accepted")
	}
	// Preserve unknown fields so the service rejects them instead of silently
	// running a different query. Raw configuration integers stay lossless.
	config, err := CheckConfig(compiled, json.RawMessage(`{"unknown":9007199254740993}`), CheckOptions{})
	if err != nil || !strings.Contains(string(config), `9007199254740993`) {
		t.Fatal(string(config), err)
	}
	_, err = New().Check(context.Background(), compiled, json.RawMessage(`{"checker":{"unsupported":true}}`), CheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = New().Check(ctx, quint.CompilationResult{}, nil, CheckOptions{}); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestCheckResponseDecoding(t *testing.T) {
	safe, err := decodeCheckResponse(&cmd.CmdResponse{Result: &cmd.CmdResponse_Success{Success: `{"modules":[{}]}`}})
	if err != nil || safe.Outcome != CheckSafe {
		t.Fatal(safe, err)
	}
	for _, status := range []string{"Error", "Deadlock", "Unknown", "ExecutionsTooShort", "SmtTimeout", "NoError"} {
		payload := `{"pass_name":"BoundedChecker","error_data":{"checking_result":"` + status + `","counterexamples":[{"states":[{"x":9007199254740993}],"#meta":{"loop":0}}]}}`
		result, err := decodeCheckResponse(&cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{Data: payload}}})
		if status == "Unknown" || status == "ExecutionsTooShort" || status == "SmtTimeout" || status == "NoError" {
			var failure *CommandFailure
			if !errors.As(err, &failure) || failure.Data != payload {
				t.Fatal("incomplete check reported as safe", result, err)
			}
			continue
		}
		want := CheckViolation
		if status == "Deadlock" {
			want = CheckDeadlock
		}
		if err != nil || result.Outcome != want || len(result.Counterexamples) != 1 {
			t.Fatal(result, err)
		}
		if !strings.Contains(string(result.Counterexamples[0]), "9007199254740993") || !strings.Contains(string(result.Counterexamples[0]), `"loop":0`) {
			t.Fatal("trace data lost")
		}
	}
	for _, response := range []*cmd.CmdResponse{nil, {},
		{Result: &cmd.CmdResponse_Success{Success: "{"}},
		{Result: &cmd.CmdResponse_Success{Success: "null"}},
		{Result: &cmd.CmdResponse_Success{Success: "{}"}},
		{Result: &cmd.CmdResponse_Success{Success: "[]"}},
		{Result: &cmd.CmdResponse_Failure{}},
		{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{Data: "{"}}},
		{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{Data: `{"pass_name":"Other"}`}}},
		{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{ErrorType: cmd.CmdErrorType_UNEXPECTED, Data: `{"msg":"failed"}`}}},
	} {
		if _, err := decodeCheckResponse(response); err == nil {
			t.Fatal("failure accepted", response)
		}
	}
}
