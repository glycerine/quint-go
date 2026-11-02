package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREPLQuietDefinitionsMultilineAndPastedOutput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input, output string
	}{
		{"values", "val x = 2\nx + 3\n.exit\n99\n", "\n5\n"},
		{"multiline", "{\nval x = 2\nx + 1\n}\n\n.exit\n", "3\n"},
		{"pasted", ">>> 1 + 2\n3\n4\n.quit\n", "3\n4\n"},
		{"comments", "// comment\n\n/* comment\nmore */\n\n1\n", "1\n"},
		{"history", "val x = 2\nx + 1\n.history\n.clear\n.history\n", "\n3\nval x = 2\nx + 1\n\n"},
		{"seed", ".seed=0x2a\n.seed\n.clear\n.seed\n", ".seed=42\n\n.seed=42\n"},
		{"exitMultiline", "{\n.exit\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := RunREPL(context.Background(), strings.NewReader(test.input), &stdout, &stderr, REPLLoopOptions{Quiet: true})
			if status != 0 || stderr.Len() != 0 || stdout.String() != test.output {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
		})
	}
}

func TestREPLErrorsKeepSessionUsable(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	input := "val x = 2\nx + true\nx\n.load\n.reload\n.save\n.verbosity=9\n.seed=-1\n.nope\n1\n"
	status := RunREPL(context.Background(), strings.NewReader(input), &stdout, &stderr, REPLLoopOptions{Quiet: true})
	if status != 0 || !strings.HasSuffix(stdout.String(), "2\n1\n") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	for _, want := range []string{".load requires", "reload", ".save is not implemented", ".verbosity requires", ".seed requires", "Unexpected command"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("missing %q in %q", want, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), "x + true") || !strings.Contains(stderr.String(), "^") {
		t.Fatalf("missing input source excerpt in %q", stderr.String())
	}
}

func TestREPLCommandsPromptsAndStartup(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	status := RunREPL(context.Background(), strings.NewReader(".help\n.verbosity=0\n1\n.exit\n"), &stdout, &stderr, REPLLoopOptions{Verbosity: 2, Commands: []string{"2 + 2"}})
	if status != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "Quint Go REPL") || !strings.Contains(stdout.String(), "4\n>>> ") || !strings.Contains(stdout.String(), ".load <filename>") || !strings.HasSuffix(stdout.String(), "1\n") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	status = RunREPL(context.Background(), &replFailureReader{}, &stdout, &stderr, REPLLoopOptions{Quiet: true, Commands: []string{".exit"}})
	if status != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("startup exit read input", status, stdout.String(), stderr.String())
	}
}

func TestREPLLoadReloadAndState(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "M.qnt")
	if err := os.WriteFile(file, []byte("module M { pure val answer = 42 }"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := RunREPL(context.Background(), strings.NewReader("answer\n.reload\nanswer\n.state\n"), &stdout, &stderr, REPLLoopOptions{Quiet: true, Require: file + "::M"})
	if status != 0 || stderr.Len() != 0 || !strings.HasPrefix(stdout.String(), "42\n42\n") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	status = RunREPL(context.Background(), strings.NewReader(".load "+file+" M\nanswer\n"), &stdout, &stderr, REPLLoopOptions{Quiet: true})
	if status != 0 || stderr.Len() != 0 || stdout.String() != "42\n" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	status = RunREPL(context.Background(), strings.NewReader(""), &stdout, &stderr, REPLLoopOptions{Quiet: true, Require: file + ".missing"})
	if status != 1 || stderr.Len() == 0 {
		t.Fatal("missing preload accepted", status)
	}
}

func TestREPLStreamFailuresAndCancellation(t *testing.T) {
	t.Parallel()
	for _, input := range []io.Reader{strings.NewReader("{\n"), &replFailureReader{}} {
		var stdout, stderr bytes.Buffer
		if status := RunREPL(context.Background(), input, &stdout, &stderr, REPLLoopOptions{Quiet: true}); status != 1 || stderr.Len() == 0 {
			t.Fatal(status, stdout.String(), stderr.String())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	if status := RunREPL(ctx, strings.NewReader("1\n"), &stdout, &stderr, REPLLoopOptions{}); status != 1 || !strings.Contains(stderr.String(), "context canceled") || stdout.Len() != 0 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	if status := RunREPL(context.Background(), strings.NewReader("1\n"), &replFailureWriter{}, &stderr, REPLLoopOptions{}); status != 1 {
		t.Fatal("writer failure", status)
	}
	if status := RunREPL(context.Background(), nil, &stdout, &stderr, REPLLoopOptions{}); status != 2 {
		t.Fatal("nil input", status)
	}
	if status := RunREPL(context.Background(), strings.NewReader(""), &stdout, &stderr, REPLLoopOptions{Verbosity: 6}); status != 2 {
		t.Fatal("invalid verbosity", status)
	}
}

func TestREPLCommandFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--unknown"}, {"--verbosity=6"}, {"--verbosity=-1"}, {"--backend=rust"}, {"--seed=oops"}} {
		var stdout, stderr bytes.Buffer
		if status := commandREPL(context.Background(), args, &stdout, &stderr); status != 2 || stderr.Len() == 0 {
			t.Fatal(args, status, stderr.String())
		}
	}
	for _, args := range [][]string{{"--help"}, {"-q", ".exit"}, {"--quiet", "--seed=42", ".exit"}} {
		var stdout, stderr bytes.Buffer
		if status := commandREPL(context.Background(), args, &stdout, &stderr); status != 0 {
			t.Fatal(args, status, stderr.String())
		}
	}
}

func TestREPLDelimiterAndDecorationRules(t *testing.T) {
	t.Parallel()
	if b, p, c := replCountBraces("{( /* */"); b != 1 || p != 1 || c != 0 {
		t.Fatal(b, p, c)
	}
	if b, p, c := replCountBraces("}) */"); b != -1 || p != -1 || c != -1 {
		t.Fatal(b, p, c)
	}
	for input, want := range map[string]string{" >>> hi": "hi", "\t... hello": "hello", "ordinary": "ordinary"} {
		if got := replTrimDecorations(input); got != want {
			t.Fatal(input, got, want)
		}
	}
}

type replFailureReader struct{}

func (*replFailureReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type replFailureWriter struct{}

func (*replFailureWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
