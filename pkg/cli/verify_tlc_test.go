package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	quint "github.com/glycerine/quint-go/pkg"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

func TestVerifyTLCOptionsAndOutput(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	dir := t.TempDir()
	config := filepath.Join(dir, "tlc.json")
	out := filepath.Join(dir, "result.json")
	if err := os.WriteFile(config, []byte(`{"workers":2,"checkDeadlock":true,"timeoutMillis":15}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"tlc", "tlc-go"} {
		called := false
		checker := verifyTLCBackend(func(_ context.Context, compiled quint.CompilationResult, opts gotlc.Options) (gotlc.Result, error) {
			called = true
			if compiled.Main != "M" || opts.Workers != 2 || !opts.CheckDeadlock || opts.MaxDuration != 15*time.Millisecond || opts.Invariants != nil {
				t.Fatal(compiled.Main, opts)
			}
			wrapper := false
			for _, d := range compiled.MainModule.Declarations {
				if d.Name == "q::inv" {
					wrapper = true
				}
			}
			if !wrapper {
				t.Fatal("invariant was not compiled")
			}
			return gotlc.Result{Complete: true, Outcome: gotlc.Safe, DistinctStates: 2}, nil
		})
		var stdout, stderr bytes.Buffer
		args := []string{input, "--backend=" + backend, "--invariant=safe", "--tlc-config", config}
		code := verifyWithCheckers(context.Background(), args, &stdout, &stderr, nil, checker)
		if code != 0 || !called || stderr.Len() != 0 || !strings.Contains(stdout.String(), "all 2 distinct states") || strings.Contains(stdout.String(), "steps") {
			t.Fatal(code, called, stdout.String(), stderr.String())
		}
		stdout.Reset()
		stderr.Reset()
		code = verifyWithCheckers(context.Background(), append(args, "--out", out), &stdout, &stderr, nil, checker)
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var stage verifyStage
		if err = json.Unmarshal(data, &stage); err != nil || code != 0 || stdout.Len() != 0 || stderr.Len() != 0 || stage.Status != "ok" || stage.TLC == nil || !stage.TLC.Complete || stage.MaxSteps != nil || len(stage.Phases) != 0 {
			t.Fatal(string(data), code, err)
		}
	}
}

func TestVerifyTLCResultsAndFailures(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	for _, tc := range []struct {
		name   string
		result gotlc.Result
		err    error
		want   string
		status string
	}{
		{"violation", gotlc.Result{Complete: true, Outcome: gotlc.Violation, Log: "x = 1"}, nil, "TLC found a violation", "violation"},
		{"deadlock", gotlc.Result{Complete: true, Outcome: gotlc.Deadlock, Log: "x = 2"}, nil, "TLC found a deadlock", "violation"},
		{"cancel", gotlc.Result{}, context.Canceled, "context canceled", "failure"},
		{"error", gotlc.Result{}, errors.New("native failure"), "native failure", "failure"},
		{"incomplete", gotlc.Result{Outcome: gotlc.Safe}, nil, "did not complete", "failure"},
		{"invalid outcome", gotlc.Result{Complete: true, Outcome: "unknown"}, nil, "invalid TLC outcome", "failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := verifyTLCBackend(func(_ context.Context, _ quint.CompilationResult, opts gotlc.Options) (gotlc.Result, error) {
				if opts.Invariants == nil || len(opts.Invariants) != 0 {
					t.Fatal(opts)
				}
				return tc.result, tc.err
			})
			var stdout, stderr bytes.Buffer
			code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc"}, &stdout, &stderr, nil, checker)
			if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatal(code, stdout.String(), stderr.String())
			}
			out := filepath.Join(t.TempDir(), "result.json")
			stdout.Reset()
			stderr.Reset()
			code = verifyWithCheckers(context.Background(), []string{input, "--backend=tlc", "--out", out}, &stdout, &stderr, nil, checker)
			data, err := os.ReadFile(out)
			var stage verifyStage
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(data, &stage); err != nil || code != 1 || stage.Status != tc.status || stage.TLC == nil || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal(code, string(data), err)
			}
		})
	}
	var stdout, stderr bytes.Buffer
	checker := verifyTLCBackend(func(context.Context, quint.CompilationResult, gotlc.Options) (gotlc.Result, error) {
		return gotlc.Result{Complete: true, Outcome: gotlc.Safe}, nil
	})
	if code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc"}, verifyBrokenWriter{}, &stderr, nil, checker); code != 1 {
		t.Fatal(code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := verifyWithCheckers(ctx, []string{input, "--backend=tlc"}, &stdout, &stderr, nil, nil); code != 1 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatal(code, stderr.String())
	}
}

func TestVerifyTLCExplicitUnsupportedOptions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--max-steps=10"}, "max-steps"},
		{[]string{"--apalache-config=x"}, "apalache-config"},
		{[]string{"--random-transitions"}, "random-transitions"},
		{[]string{"--inductive-invariant=safe"}, "inductive-invariant"},
		{[]string{"--temporal=live"}, "temporal"},
		{[]string{"--out-itf=trace.json"}, "out-itf"},
	} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"M.qnt", "--backend=tlc"}, tc.args...)
		options, help, err := verifyParseOptions(args, &stdout, &stderr)
		if err == nil || help || !strings.Contains(err.Error(), tc.want) {
			t.Fatal(options, help, err)
		}
	}
	for _, input := range []string{"missing.json", "invalid.json", "jvm.json"} {
		dir := t.TempDir()
		name := filepath.Join(dir, input)
		if input == "invalid.json" {
			if err := os.WriteFile(name, []byte(`[]`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if input == "jvm.json" {
			if err := os.WriteFile(name, []byte(`{"maxHeap":"8G"}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		var stdout, stderr bytes.Buffer
		code := verifyWithCheckers(context.Background(), []string{"M.qnt", "--backend=tlc", "--tlc-config", name}, &stdout, &stderr, nil, nil)
		if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "TLC configuration") {
			t.Fatal(code, stderr.String())
		}
	}
}

func TestVerifyTLCActualGoChecker(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC source CLI corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		safe     string
		wantCode int
		want     string
	}{{"x>=0", 0, "all 2 distinct states"}, {"x==0", 1, "TLC found a violation"}} {
		input := verifyTestSource(t, "module M { var x:int action init=x'=0 action step=x'=1-x val safe="+tc.safe+" }")
		var stdout, stderr bytes.Buffer
		code := commandVerify(context.Background(), []string{input, "--backend=tlc", "--invariant=safe"}, &stdout, &stderr)
		if code != tc.wantCode || !strings.Contains(stdout.String()+stderr.String(), tc.want) {
			t.Fatal(code, stdout.String(), stderr.String())
		}
		out := filepath.Join(t.TempDir(), "result.json")
		code = commandVerify(context.Background(), []string{input, "--backend=tlc", "--invariant=safe", "--out", out}, io.Discard, io.Discard)
		data, err := os.ReadFile(out)
		var stage verifyStage
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &stage); err != nil || code != tc.wantCode || stage.TLC == nil || !stage.TLC.Complete {
			t.Fatal(code, string(data), err)
		}
		if tc.wantCode == 1 && (len(stage.TLC.Trace) != 2 || stage.TLC.Trace[0].Values[0].Value != "0" || stage.TLC.Trace[1].Values[0].Value != "1") {
			t.Fatal(string(data))
		}
	}
}
