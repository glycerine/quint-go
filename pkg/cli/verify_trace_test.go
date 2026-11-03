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

	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamVerificationCounterExampleDisplay(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Verify []struct {
			Name, Status, Explanation, TraceOutput string
			Verbosity                              int
			TraceITF                               json.RawMessage
		}
	}
	data, err := os.ReadFile("../testdata/parser/trace_display.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Verify {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			stage := verifyStage{Status: fixture.Status, Explanation: fixture.Explanation, Trace: fixture.TraceITF, Errors: []quint.ErrorMessage{}}
			options := verifyOptions{verbosity: fixture.Verbosity}
			var out, stderr bytes.Buffer
			if err := verifyPrintCounterExample(context.Background(), &out, options, stage); err != nil || out.String() != fixture.TraceOutput {
				t.Fatalf("got %q err=%v\nwant %q", out.String(), err, fixture.TraceOutput)
			}
			out.Reset()
			code := verifyOutputContext(context.Background(), options, stage, &out, &stderr)
			wantCode := 1
			if stage.Status == "ok" {
				wantCode = 0
			}
			if code != wantCode || !strings.HasPrefix(out.String(), fixture.TraceOutput) {
				t.Fatal("verification did not display the trace before its report", code, out.String(), stderr.String())
			}
		})
	}
}

func TestVerificationTraceDisplayErrorsAndSuppression(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	valid := json.RawMessage(`{"states":[{"x":{"#bigint":"1"}}]}`)
	stage := verifyStage{Status: "violation", Trace: valid, Explanation: "counterexample", Errors: []quint.ErrorMessage{}}
	var out, stderr bytes.Buffer
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := verifyPrintCounterExample(canceled, &out, verifyOptions{verbosity: 2}, stage); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal(out.String(), err)
	}
	for _, trace := range []json.RawMessage{nil, json.RawMessage(`{`)} {
		invalid := stage
		invalid.Trace = trace
		if trace == nil {
			if err := verifyPrintCounterExample(ctx, &out, verifyOptions{verbosity: 2}, invalid); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := verifyPrintCounterExample(ctx, &out, verifyOptions{verbosity: 2}, invalid); err == nil || out.Len() != 0 {
			t.Fatal("malformed trace was partially displayed", out.String(), err)
		}
		if status := verifyOutputContext(ctx, verifyOptions{verbosity: 2}, invalid, &out, &stderr); status != 1 || !strings.Contains(stderr.String(), "display counterexample") {
			t.Fatal(status, out.String(), stderr.String())
		}
		stderr.Reset()
		if err := verifyPrintCounterExample(ctx, &out, verifyOptions{verbosity: 1}, invalid); err != nil {
			t.Fatal("disabled trace output decoded its input", err)
		}
	}
	failure := errors.New("trace output failed")
	for _, write := range []int{1, 2} {
		for _, short := range []bool{false, true} {
			writer := &executionFailWriter{failAt: write, err: failure, short: short}
			err := verifyPrintCounterExample(ctx, writer, verifyOptions{verbosity: 2}, stage)
			if !errors.Is(err, failure) && !errors.Is(err, io.ErrShortWrite) {
				t.Fatal("trace output lost writer failure", write, err)
			}
		}
	}
	for _, kind := range []string{"json", "itf"} {
		options := verifyOptions{verbosity: 5}
		name := filepath.Join(t.TempDir(), "trace.json")
		if kind == "json" {
			options.out = name
		} else {
			options.outITF = name
		}
		out.Reset()
		stderr.Reset()
		if code := verifyOutputContext(ctx, options, stage, &out, &stderr); code != 1 || out.Len() != 0 || stderr.Len() != 0 {
			t.Fatal("file output did not suppress console trace", kind, code, out.String(), stderr.String())
		}
	}
}

func TestCLIAnnotatedTraceMatchesSharedFormatter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := quint.Expr{Kind: "app", Opcode: "Rec", Args: []quint.Expr{}}
	options := quint.TraceFormatOptions{Diagnostics: [][]quint.DebugMessage{{{Label: "state", Value: quint.Expr{Kind: "int"}}}}, PendingDiagnostics: []quint.DebugMessage{{Label: "pending", Value: quint.Expr{Kind: "bool", Bool: true}}}}
	want, err := quint.FormatTraceContext(ctx, []quint.Expr{state}, 80, options)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := executionPrintAnnotatedTrace(ctx, &out, []quint.Expr{state}, options); err != nil || out.String() != want {
		t.Fatal(out.String(), err, want)
	}
	if err := executionPrintAnnotatedTrace(ctx, io.Discard, []quint.Expr{{Kind: "bool"}}, options); err == nil {
		t.Fatal("malformed annotated trace displayed")
	}
	writer := &executionFailWriter{failAt: 1, err: errors.New("empty output must not write")}
	if err := executionPrintAnnotatedTrace(ctx, writer, nil, quint.TraceFormatOptions{}); err != nil || writer.calls != 0 {
		t.Fatal("empty trace wrote to its output stream", err, writer.calls)
	}
}
