package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type executionFramesFixtureFrame struct {
	App       quint.Expr
	Args      []quint.Expr
	Result    *quint.Expr
	Error     *quint.Diagnostic
	Subframes []executionFramesFixtureFrame
}

type executionFramesFixtures struct {
	Run []struct {
		Name      string
		Verbosity int
		States    []quint.Expr
		Frames    []executionFramesFixtureFrame
		Hide      []string
		Output    string
	}
	Test []struct {
		Name, Input string
		Verbosity   int
		Code        []struct{ Source, Text string }
		Failures    []struct {
			Name, Seed string
			Messages   []quint.ErrorMessage
			Frames     []executionFramesFixtureFrame
		}
		Output string
	}
	Repl []struct {
		Name, Input, FramesOutput string
		Verbosity                 int
	}
}

func executionLoadFixtures(t *testing.T) executionFramesFixtures {
	t.Helper()
	data, err := os.ReadFile("../testdata/parser/cli_execution_frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var result executionFramesFixtures
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func executionConvertFrames(t *testing.T, input []executionFramesFixtureFrame) []quint.ExecutionFrame {
	t.Helper()
	frames := make([]quint.ExecutionFrame, len(input))
	for i, fixture := range input {
		frame := &frames[i]
		frame.App = fixture.App
		for _, expression := range fixture.Args {
			value, err := quint.NewRuntimeValue(expression)
			if err != nil {
				t.Fatal(err)
			}
			frame.Args = append(frame.Args, value)
		}
		if fixture.Error != nil {
			frame.Err = *fixture.Error
		} else {
			var err error
			frame.Result, err = quint.NewRuntimeValue(*fixture.Result)
			if err != nil {
				t.Fatal(err)
			}
		}
		frame.Subframes = executionConvertFrames(t, fixture.Subframes)
	}
	return frames
}

func TestUpstreamCLIExecutionFrameReports(t *testing.T) {
	t.Parallel()
	fixtures := executionLoadFixtures(t)
	for _, fixture := range fixtures.Run {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			before, _ := json.Marshal(fixture.States)
			frames := executionConvertFrames(t, fixture.Frames)
			err := executionPrintCounterExample(context.Background(), &out, fixture.Verbosity, fixture.States, frames, fixture.Hide, nil)
			if err != nil || out.String() != fixture.Output {
				t.Fatalf("got %q (%v)\nwant %q", out.String(), err, fixture.Output)
			}
			after, _ := json.Marshal(fixture.States)
			if !bytes.Equal(before, after) {
				t.Fatal("report changed state fields")
			}
		})
	}
	for _, fixture := range fixtures.Test {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			code := insdict.NewDict[string, string]()
			for _, source := range fixture.Code {
				code.Set(source.Source, source.Text)
			}
			failures := make([]executionTestFailure, len(fixture.Failures))
			for i, failure := range fixture.Failures {
				seed, err := quint.ParseInteger(failure.Seed)
				if err != nil {
					t.Fatal(err)
				}
				failures[i] = executionTestFailure{Name: failure.Name, Seed: seed, Messages: failure.Messages, Frames: executionConvertFrames(t, failure.Frames)}
			}
			var out bytes.Buffer
			err := executionPrintTestErrors(context.Background(), &out, fixture.Verbosity, fixture.Input, code, failures, nil, quint.DiagnosticRenderOptions{InclusiveEnd: true})
			if err != nil || out.String() != fixture.Output {
				t.Fatalf("got %q (%v)\nwant %q", out.String(), err, fixture.Output)
			}
		})
	}
}

func TestCLIExecutionFrameSelectionAndErrors(t *testing.T) {
	t.Parallel()
	truth, _ := quint.NewRuntimeValue(quint.Expr{Kind: "bool", Bool: true})
	frame := quint.ExecutionFrame{App: quint.Expr{Kind: "app", Opcode: "child"}, Result: truth}
	wrapped := []quint.ExecutionFrame{{App: quint.Expr{Kind: "app", Opcode: "private"}, Subframes: []quint.ExecutionFrame{{App: quint.Expr{Kind: "app", Opcode: "private"}, Subframes: []quint.ExecutionFrame{frame}}}}, frame}
	if got := executionUnwrapFrames(wrapped, "private"); len(got) != 2 || got[0].App.Opcode != "child" || wrapped[0].App.Opcode != "private" {
		t.Fatal("private entry unwrapping", got)
	}
	if !reflect.DeepEqual(executionUnwrapFrames(wrapped, ""), wrapped) || executionBestFrames(nil, "") != nil {
		t.Fatal("empty recording selection")
	}
	recorder := quint.NewTraceRecorder(3, nil)
	if executionBestFrames(recorder, "") != nil {
		t.Fatal("selected an unfinished trace")
	}
	recorder.OnUserOperatorCall(frame.App)
	recorder.OnUserOperatorReturn(frame.App, nil, truth, nil)
	recorder.OnRunReturn(truth, nil, nil)
	if frames := executionBestFrames(recorder, ""); len(frames) != 1 || frames[0].App.Opcode != "child" {
		t.Fatal("best recorded frames", frames)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executionPrintFrames(ctx, io.Discard, nil, nil, false); !errors.Is(err, context.Canceled) {
		t.Fatal("empty frame report ignored cancellation", err)
	}
	if err := executionPrintTrace(ctx, io.Discard, []quint.Expr{{Kind: "app", Opcode: "Rec"}}, nil, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("state report ignored cancellation", err)
	}
	if err := executionPrintFrames(context.Background(), io.Discard, []quint.ExecutionFrame{{}}, nil, false); err == nil {
		t.Fatal("invalid frame printed")
	}
	if err := executionPrintTrace(context.Background(), io.Discard, []quint.Expr{{Kind: "bool"}}, nil, nil, nil); err == nil {
		t.Fatal("invalid state printed")
	}
	if err := executionPrintFrames(context.Background(), io.Discard, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	message := quint.ErrorMessage{Explanation: "[QNT501] failed"}
	options := []executionTestFailure{{Name: "failure", Messages: []quint.ErrorMessage{message}, Frames: []quint.ExecutionFrame{frame}, Repeat: "Go replay"}}
	var out bytes.Buffer
	if err := executionPrintTestErrors(context.Background(), &out, 3, "source.qnt", nil, options, nil); err != nil || !strings.Contains(out.String(), "    Go replay\n") {
		t.Fatal("caller replay instruction", out.String(), err)
	}
	verifyWriterErrors := func(render func(io.Writer) error) {
		t.Helper()
		writer := &executionFailWriter{}
		if err := render(writer); err != nil {
			t.Fatal(err)
		}
		for index := 1; index <= writer.calls; index++ {
			failure := errors.New("write failed")
			if err := render(&executionFailWriter{failAt: index, err: failure}); !errors.Is(err, failure) {
				t.Fatalf("write %d error was lost: %v", index, err)
			}
			if err := render(&executionFailWriter{failAt: index, short: true}); !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("short write %d was lost: %v", index, err)
			}
		}
	}
	verifyWriterErrors(func(out io.Writer) error {
		return executionPrintFrames(context.Background(), out, []quint.ExecutionFrame{frame}, nil, false)
	})
	verifyWriterErrors(func(out io.Writer) error { return executionPrintFrames(context.Background(), out, nil, nil, true) })
	verifyWriterErrors(func(out io.Writer) error {
		return executionPrintCounterExample(context.Background(), out, 3, []quint.Expr{{Kind: "app", Opcode: "Rec"}}, []quint.ExecutionFrame{frame}, nil, nil)
	})
	for _, level := range []int{2, 3} {
		verifyWriterErrors(func(out io.Writer) error {
			return executionPrintTestErrors(context.Background(), out, level, "source.qnt", nil, options, nil)
		})
	}
}

type executionFailWriter struct {
	calls, failAt int
	err           error
	short         bool
}

func (w *executionFailWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		if w.short {
			return 0, nil
		}
		return 0, w.err
	}
	return len(data), nil
}
