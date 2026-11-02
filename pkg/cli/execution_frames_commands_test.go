package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestCLIExecutionFramesRunSourceTranscripts(t *testing.T) {
	t.Parallel()
	fixtures := executionLoadFixtures(t)
	input := runTestSource(t, "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x<=1 }")
	for _, fixture := range fixtures.Run {
		if !strings.HasPrefix(fixture.Name, "violation_") {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			status := commandRun(context.Background(), []string{input, "--seed=17", "--max-steps=3", "--max-samples=2", "--n-traces=2", "--invariant=safe", fmt.Sprintf("--verbosity=%d", fixture.Verbosity)}, &out, &stderr)
			if status != 1 || !strings.Contains(stderr.String(), "Invariant violated") {
				t.Fatal(status, out.String(), stderr.String())
			}
			text := out.String()
			if at := strings.Index(text, "[violation]"); at >= 0 {
				text = text[:at]
			}
			if text != fixture.Output {
				t.Fatalf("got %q\nwant %q", text, fixture.Output)
			}
		})
	}
}

func TestCLIExecutionFramesREPLSourceTranscripts(t *testing.T) {
	t.Parallel()
	for _, fixture := range executionLoadFixtures(t).Repl {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			if status := RunREPL(context.Background(), strings.NewReader(fixture.Input), &out, &stderr, REPLLoopOptions{Verbosity: fixture.Verbosity}); status != 0 {
				t.Fatal(status, out.String(), stderr.String())
			}
			text := out.String()
			frames := ""
			if at := strings.Index(text, "\n[Frame "); at >= 0 {
				frames = text[at:]
				if end := strings.Index(frames, ">>> "); end >= 0 {
					frames = frames[:end]
				}
			}
			if frames != fixture.FramesOutput {
				t.Fatalf("got %q\nwant %q\nstdout %q stderr %q", frames, fixture.FramesOutput, out.String(), stderr.String())
			}
		})
	}
}

func TestCLIExecutionFramesREPLReplayAndErrorOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := quint.NewREPLSession()
	for _, input := range []string{"pure def inc(x)=x+1", "inc(1)", "inc(2)"} {
		if _, err := session.Evaluate(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(t.TempDir(), "saved.qnt")
	if err := session.Save(ctx, file); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if status := RunREPL(ctx, strings.NewReader(".reload\n.exit\n"), &out, &stderr, REPLLoopOptions{Verbosity: 3, Require: file}); status != 0 || stderr.Len() != 0 {
		t.Fatal(status, out.String(), stderr.String())
	}
	if strings.Count(out.String(), "[Frame 0]\ninc(") != 4 || strings.Contains(out.String(), "[Frame 1]") || strings.Contains(out.String(), "q::input =>") {
		t.Fatal("saved replay retained earlier frames or exposed a private entry", out.String())
	}
	out.Reset()
	// A shared stream makes relative order observable across stdout/stderr.
	input := "pure def boom(x)=List().head()\nboom(1)\n.exit\n"
	if status := RunREPL(ctx, strings.NewReader(input), &out, &out, REPLLoopOptions{Verbosity: 3}); status != 0 {
		t.Fatal(status, out.String())
	}
	frameAt, errorAt := strings.Index(out.String(), "[Frame 0]"), strings.Index(out.String(), "Error [QNT505]")
	if frameAt < 0 || errorAt < frameAt {
		t.Fatal("runtime error was printed before its frames", out.String())
	}
	// A JSON stage request suppresses frame recording and terminal output even
	// when verbosity is explicitly high.
	stage := filepath.Join(t.TempDir(), "stage.json")
	file = testCommandFile(t, "module M { action failTest=false }")
	status, stdout, errors := testCommandCall(t, ctx, file, "--seed=0", "--verbosity=5", "--out", stage)
	if status != 1 || stdout != "" || errors != "" {
		t.Fatal(status, stdout, errors)
	}
	if data, err := os.ReadFile(stage); err != nil || bytes.Contains(data, []byte("subframes")) {
		t.Fatal("execution frames leaked into the established JSON stage", string(data), err)
	}
}

func TestCLIExecutionFramesTestErrorSourceTranscripts(t *testing.T) {
	t.Parallel()
	fixtures := executionLoadFixtures(t)
	input := testCommandFile(t, "module M { action failTest=false }")
	for _, fixture := range fixtures.Test {
		if !strings.HasPrefix(fixture.Name, "false_action_") && fixture.Verbosity != 3 {
			continue
		}
		caseInput := input
		if !strings.HasPrefix(fixture.Name, "false_action_") {
			caseInput = testCommandFile(t, fixture.Code[0].Text)
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			status, out, stderr := testCommandCall(t, context.Background(), caseInput, "--seed=17", fmt.Sprintf("--verbosity=%d", fixture.Verbosity))
			if status != 1 || stderr != "error: Tests failed\n" {
				t.Fatal(status, out, stderr)
			}
			text := ""
			if at := strings.Index(out, "\n  1) failTest:"); at >= 0 {
				text = out[at:]
			}
			want := strings.ReplaceAll(fixture.Output, fixture.Input, caseInput)
			want = strings.ReplaceAll(want, "--match=failTest to repeat.", "--match=^failTest$ --backend=go to repeat.")
			if text != want {
				t.Fatalf("got %q\nwant %q", text, want)
			}
		})
	}
}
