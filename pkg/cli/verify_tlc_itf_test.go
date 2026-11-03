package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

func TestVerifyTLCITFOutputAndFailures(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	trace := json.RawMessage(`{"vars":["x"],"states":[{"#meta":{"index":0},"x":{"#bigint":"2147483647"}}],"loop":0}`)
	for _, tc := range []struct {
		name    string
		outcome gotlc.Outcome
		trace   json.RawMessage
		want    int
		status  string
	}{
		{"safe", gotlc.Safe, nil, 0, "ok"},
		{"violation", gotlc.Violation, trace, 1, "violation"},
		{"deadlock", gotlc.Deadlock, trace, 1, "violation"},
		{"missing ITF", gotlc.Violation, nil, 1, "failure"},
		{"invalid ITF", gotlc.Violation, json.RawMessage(`{`), 1, "failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			output, itf := filepath.Join(dir, "result.json"), filepath.Join(dir, "trace.itf.json")
			checker := verifyTLCBackend(func(_ context.Context, _ quint.CompilationResult, opts gotlc.Options) (gotlc.Result, error) {
				if !opts.ITF {
					t.Fatal("ITF conversion was not requested")
				}
				return gotlc.Result{Outcome: tc.outcome, Complete: true, ITF: tc.trace}, nil
			})
			var stdout, stderr bytes.Buffer
			code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc-go", "--out", output, "--out-itf", itf}, &stdout, &stderr, nil, checker)
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var stage verifyStage
			if err := json.Unmarshal(data, &stage); err != nil || code != tc.want || stage.Status != tc.status || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal(code, string(data), stdout.String(), stderr.String(), err)
			}
			itfData, err := os.ReadFile(itf)
			if tc.status == "violation" {
				if err != nil || !bytes.Equal(bytes.TrimSpace(itfData), trace) {
					t.Fatal(string(itfData), err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("unexpected ITF output", string(itfData), err)
			}
			if stage.Explanation != "" || len(stage.Trace) != 0 {
				t.Fatal("TLC stage leaked backend fields", string(data))
			}
		})
	}
	checker := verifyTLCBackend(func(_ context.Context, _ quint.CompilationResult, opts gotlc.Options) (gotlc.Result, error) {
		return gotlc.Result{Outcome: gotlc.Violation, Complete: true, ITF: trace}, nil
	})
	var stdout, stderr bytes.Buffer
	if code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc", "--out-itf", filepath.Join(t.TempDir(), "missing", "trace.json")}, &stdout, &stderr, nil, checker); code != 1 || !strings.Contains(stderr.String(), "write counterexample") {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	checker = verifyTLCBackend(func(context.Context, quint.CompilationResult, gotlc.Options) (gotlc.Result, error) {
		return gotlc.Result{Outcome: gotlc.Violation, Complete: true, ITFError: "conversion unavailable"}, nil
	})
	stdout.Reset()
	stderr.Reset()
	dir := t.TempDir()
	output := filepath.Join(dir, "result.json")
	if code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc", "--out", output, "--out-itf", filepath.Join(dir, "trace.json")}, &stdout, &stderr, nil, checker); code != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var stage verifyStage
	if err := json.Unmarshal(data, &stage); err != nil || stage.Status != "failure" || stage.Explanation != "" {
		t.Fatal(string(data), err)
	}
}

func TestVerifyTLCActualITFCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC source CLI ITF corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		name, body, flags string
		code              int
		loop              bool
	}{
		{"safe", "val safe=true", "safe", 0, false},
		{"next-state violation", "val safe=not(x)", "safe", 1, false},
		{"initial violation", "val safe=x", "safe", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := verifyTestSource(t, `module M { var x:bool action init=x'=false action step=x'=not(x) `+tc.body+` }`)
			path := filepath.Join(t.TempDir(), "trace.itf.json")
			var stdout, stderr bytes.Buffer
			code := commandVerify(context.Background(), []string{input, "--backend=tlc-go", "--invariant=" + tc.flags, "--out-itf", path}, &stdout, &stderr)
			if code != tc.code || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal(code, stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(path)
			if code == 0 {
				if !os.IsNotExist(err) {
					t.Fatal(string(data), err)
				}
				return
			}
			trace, err := quint.DecodeITFTrace(data)
			if err != nil || trace.Loop != nil || len(trace.Vars) != 1 || trace.Vars[0] != "x" || len(trace.States) == 0 {
				t.Fatal(string(data), err)
			}
			if tc.name == "next-state violation" && len(trace.States) != 2 || tc.name == "initial violation" && len(trace.States) != 1 {
				t.Fatal(string(data))
			}
			for i, state := range trace.States {
				if len(state.Args) != 2 || state.Args[0].Str != "x" || state.Args[1].Kind != "bool" || state.Args[1].Bool != (i > 0) {
					t.Fatal(string(data))
				}
			}
		})
	}
}
