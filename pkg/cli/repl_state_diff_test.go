package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamREPLStateDiffTranscripts(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, Input, Require, Output, Stderr string
			Verbosity                            int
			Files                                []struct{ Name, Source string }
		}
	}
	encoded, err := os.ReadFile("../testdata/parser/repl_state_diff.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	prompts := regexp.MustCompile(`(?m)^(?:(?:>>> |\.\.\. ))+`)
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for _, file := range fixture.Files {
				if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			input := strings.ReplaceAll(fixture.Input, "$DIR", directory)
			require := strings.ReplaceAll(fixture.Require, "$DIR", directory)
			seed, err := quint.ParseSeed("17")
			if err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			status := RunREPL(context.Background(), strings.NewReader(input), &out, &stderr, REPLLoopOptions{Verbosity: fixture.Verbosity, Require: require, Seed: &seed})
			text := strings.TrimPrefix(out.String(), "Quint Go REPL\nType \".exit\" to exit, or \".help\" for more information\n")
			text = prompts.ReplaceAllString(text, "")
			text = strings.ReplaceAll(text, directory, "$DIR")
			if status != 0 || text != fixture.Output || stderr.String() != fixture.Stderr {
				t.Fatalf("status=%d stdout=%q\nwant=%q\nstderr=%q want=%q", status, text, fixture.Output, stderr.String(), fixture.Stderr)
			}
		})
	}
}

func TestREPLStateDiffFormattingAndFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := quint.NewREPLSession()
	if _, err := session.Evaluate(ctx, "var x:int"); err != nil {
		t.Fatal(err)
	}
	result, err := session.Evaluate(ctx, "x'=1")
	if err != nil {
		t.Fatal(err)
	}
	for level := 0; level <= 5; level++ {
		var out bytes.Buffer
		err := replPrintStateDiff(ctx, &out, result, level)
		want := ""
		if level >= 2 {
			want = "{ x: 1 }\n"
		}
		if err != nil || out.String() != want {
			t.Fatal(level, out.String(), err)
		}
	}
	if err := replPrintStateDiff(ctx, io.Discard, quint.REPLResult{}, 2); err != nil {
		t.Fatal("unshifted values need no state snapshots", err)
	}
	if err := replPrintStateDiff(ctx, io.Discard, quint.REPLResult{Shifted: true}, 2); err == nil {
		t.Fatal("invalid commit snapshots were displayed")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, level := range []int{0, 2} {
		if err := replPrintStateDiff(canceled, io.Discard, result, level); !errors.Is(err, context.Canceled) {
			t.Fatal("diff display ignored cancellation", err)
		}
	}
	failure := errors.New("diff writer failed")
	for _, writer := range []io.Writer{&executionFailWriter{failAt: 1, err: failure}, &executionFailWriter{failAt: 1, short: true}} {
		err := replPrintStateDiff(ctx, writer, result, 2)
		if !errors.Is(err, failure) && !errors.Is(err, io.ErrShortWrite) {
			t.Fatal("diff display lost writer error", err)
		}
	}
}

func TestREPLQuietStateCommitsAndDiffWriterFailure(t *testing.T) {
	t.Parallel()
	input := "var x:int\nx'=1\nx\n.exit\n"
	var out, stderr bytes.Buffer
	status := RunREPL(context.Background(), strings.NewReader(input), &out, &stderr, REPLLoopOptions{Quiet: true, Verbosity: 5})
	if status != 0 || stderr.Len() != 0 || out.String() != "\ntrue\n1\n" {
		t.Fatal("quiet mode must still commit values without diffs", status, out.String(), stderr.String())
	}
	failure := errors.New("diff writer failed")
	stderr.Reset()
	status = RunREPL(context.Background(), strings.NewReader(input), replDiffFailWriter{failure}, &stderr, REPLLoopOptions{Verbosity: 2})
	if status != 1 {
		t.Fatal("diff writer failure did not stop the REPL", status, stderr.String())
	}
}

func TestREPLStateDiffReportsCheckedDisplayErrorsOnce(t *testing.T) {
	t.Parallel()
	// An infinite product can be retained in a variable, but displaying its
	// elements must return the checked enumeration error after the true result.
	input := "var x:Set[(int, int)]\nx'=tuples(Int, Set(1))\n42\n.exit\n"
	var out, stderr bytes.Buffer
	status := RunREPL(context.Background(), strings.NewReader(input), &out, &stderr, REPLLoopOptions{Verbosity: 2})
	if status != 0 || !strings.Contains(out.String(), "true\n") || !strings.Contains(out.String(), "42\n") || strings.Count(stderr.String(), "non-enumerable") != 1 {
		t.Fatal("checked diff display errors must be reported once and preserve the session", status, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	status = RunREPL(context.Background(), strings.NewReader(input), &out, &stderr, REPLLoopOptions{Quiet: true})
	if status != 0 || stderr.Len() != 0 || out.String() != "\ntrue\n42\n" {
		t.Fatal("disabled diffs must not enumerate stored domains", status, out.String(), stderr.String())
	}
}

type replDiffFailWriter struct{ failure error }

func (w replDiffFailWriter) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("{ x:")) {
		return 0, w.failure
	}
	return len(data), nil
}
