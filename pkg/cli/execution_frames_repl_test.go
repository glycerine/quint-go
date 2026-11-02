package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestCLIExecutionFramesREPLContextAndReplayErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := quint.NewREPLSession()
	plain, recorder, err := executionREPLContext(ctx, session, 2)
	if err != nil || plain != ctx || recorder != nil {
		t.Fatal("default REPL installed a recorder", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := executionREPLContext(canceled, session, 3); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := executionREPLContext(ctx, nil, 3); err == nil {
		t.Fatal("uninitialized session accepted recording")
	}
	if err := executionWrite(canceled, io.Discard, "unwritten"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	failure := errors.New("display failed")
	for _, level := range []int{2, 3} {
		var reported []error
		var writeErr error
		report := func(err error) { reported = append(reported, err) }
		render := func(quint.REPLResult) error { return failure }
		_, hooks, err := executionREPLReplay(ctx, session, func() int { return level }, func(string) {}, io.Discard, report, render, &writeErr)
		if err != nil {
			t.Fatal(err)
		}
		if err := hooks.After(quint.REPLReplayResult{}); err != nil || len(reported) != 1 || !errors.Is(reported[0], failure) {
			t.Fatal("replay lost or duplicated a display error", reported, err)
		}
	}
	var writeErr error
	if _, _, err := executionREPLReplay(canceled, session, func() int { return 3 }, func(string) {}, io.Discard, func(error) {}, func(quint.REPLResult) error { return nil }, &writeErr); !errors.Is(err, context.Canceled) {
		t.Fatal("replay ignored cancellation", err)
	}
	var reported []error
	recordingContext, hooks, err := executionREPLReplay(ctx, session, func() int { return 3 }, func(string) {}, io.Discard, func(err error) { reported = append(reported, err) }, func(quint.REPLResult) error { return nil }, &writeErr)
	if err != nil {
		t.Fatal(err)
	}
	listener := quint.ExecutionListenerFromContext(recordingContext)
	listener.OnUserOperatorCall(quint.Expr{Kind: "app", Opcode: "bad"})
	listener.OnUserOperatorReturn(quint.Expr{}, nil, quint.RuntimeValue{}, nil)
	if err := hooks.After(quint.REPLReplayResult{}); err != nil || len(reported) != 1 {
		t.Fatal("frame display error was lost", reported, err)
	}
	if err := hooks.Before("saved.qnt", "1"); err != nil {
		t.Fatal(err)
	}
	reported = nil
	if err := hooks.After(quint.REPLReplayResult{Err: failure}); err != nil || len(reported) != 1 || !errors.Is(reported[0], failure) {
		t.Fatal("replay evaluation error was lost", reported, err)
	}
}

func TestCLIExecutionFramesREPLCheckedValuesAndWriterFailures(t *testing.T) {
	t.Parallel()
	input := "pure def identity(x)=x\nidentity(1.to(3))\n.exit\n"
	var out, stderr bytes.Buffer
	if status := RunREPL(context.Background(), strings.NewReader(input), &out, &stderr, REPLLoopOptions{Verbosity: 3}); status != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "identity(Set(1, 2, 3)) => Set(1, 2, 3)") {
		t.Fatal("compact frame values did not use checked serialization", status, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	input = "pure def identity(x)=x\nidentity(tuples(Int))\n.exit\n"
	if status := RunREPL(context.Background(), strings.NewReader(input), &out, &stderr, REPLLoopOptions{Verbosity: 3}); status != 0 || strings.Count(stderr.String(), "non-enumerable") != 1 {
		t.Fatal("non-enumerable display errors must be reported once", status, out.String(), stderr.String())
	}
	failure := errors.New("frame writer failed")
	if status := RunREPL(context.Background(), strings.NewReader("pure def inc(x)=x+1\ninc(1)\n.exit\n"), executionFrameFailWriter{failure}, io.Discard, REPLLoopOptions{Verbosity: 3}); status != 1 {
		t.Fatal("frame write failure did not stop the REPL", status)
	}
}

type executionFrameFailWriter struct{ failure error }

func (w executionFrameFailWriter) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("[Frame ")) {
		return 0, w.failure
	}
	return len(data), nil
}
