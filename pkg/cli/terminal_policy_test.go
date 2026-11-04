package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

var terminalTestANSI = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

func TestTerminalREPLWidthColorsAndDiagnostics(t *testing.T) {
	t.Parallel()
	seed, err := quint.ParseSeed("17")
	if err != nil {
		t.Fatal(err)
	}
	run := func(width, level int, input string) (int, string, string) {
		ctx := WithTerminalOptions(context.Background(), TerminalOptions{Width: width, ColorLevel: &level})
		var stdout, stderr bytes.Buffer
		status := RunREPL(ctx, strings.NewReader(input), &stdout, &stderr, REPLLoopOptions{Seed: &seed})
		return status, stdout.String(), stderr.String()
	}
	status, narrow, stderr := run(8, 0, "List(1,2,3)\n.exit\n")
	if status != 0 || stderr != "" || narrow != "[\n  1,\n  2,\n  3\n]\n" {
		t.Fatalf("narrow status=%d stdout=%q stderr=%q", status, narrow, stderr)
	}
	status, colored, stderr := run(8, 1, "List(1,2,3)\n.exit\n")
	if status != 0 || stderr != "" || terminalTestANSI.ReplaceAllString(colored, "") != narrow || !strings.Contains(colored, "\x1b[33m1\x1b[39m") {
		t.Fatalf("colored status=%d stdout=%q stderr=%q", status, colored, stderr)
	}
	status, wide, stderr := run(80, 0, "List(1,2,3)\n.exit\n")
	if status != 0 || stderr != "" || wide != "[1, 2, 3]\n" {
		t.Fatalf("wide=%q status=%d stderr=%q", wide, status, stderr)
	}
	status, debug, stderr := run(8, 1, "q::debug(\"label\", List(1,2,3))\n.exit\n")
	if status != 0 || stderr != "" || !strings.HasPrefix(debug, "> label [\n  \x1b[33m1") {
		t.Fatalf("debug width/color status=%d stdout=%q stderr=%q", status, debug, stderr)
	}
	status, _, stderr = run(80, 1, "missing\n.exit\n")
	if status != 0 || !strings.Contains(stderr, "\x1b[31m") || !strings.Contains(stderr, "QNT404") || !strings.HasSuffix(stderr, "\x1b[31m\x1b[39m") {
		t.Fatalf("error color status=%d stderr=%q", status, stderr)
	}
}

func TestTerminalFrontendDiagnosticsAndJSONOwnership(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	input := filepath.Join(directory, "bad.qnt")
	if err := os.WriteFile(input, []byte("module bad { pure val value = missing }"), 0600); err != nil {
		t.Fatal(err)
	}
	level := 1
	ctx := WithTerminalOptions(context.Background(), TerminalOptions{ColorLevel: &level})
	var stdout, stderr bytes.Buffer
	if status := Execute(ctx, []string{"typecheck", input}, &stdout, &stderr); status != 1 || stdout.Len() != 0 || strings.Contains(stderr.String(), "\x1b[31m") || !strings.Contains(stderr.String(), "QNT404") {
		t.Fatalf("diagnostic status=%d out=%q err=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	output := filepath.Join(directory, "stage.json")
	if status := Execute(ctx, []string{"typecheck", input, "--out", output}, &stdout, &stderr); status != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("JSON status=%d out=%q err=%q", status, stdout.String(), stderr.String())
	}
	encoded, err := os.ReadFile(output)
	if err != nil || !json.Valid(encoded) || bytes.Contains(encoded, []byte{0x1b}) || bytes.Contains(encoded, []byte(`\u001b`)) {
		t.Fatalf("JSON gained terminal styles: %s (%v)", encoded, err)
	}
}

func TestTerminalVerifyReportsUseInvocationContext(t *testing.T) {
	t.Parallel()
	level := 1
	ctx := WithTerminalOptions(context.Background(), TerminalOptions{ColorLevel: &level})
	var stdout, stderr bytes.Buffer
	if status := verifyOutputContext(ctx, verifyOptions{verbosity: 1, maxSteps: 2}, verifyStage{Status: "ok"}, &stdout, &stderr); status != 0 || stdout.String() != "\x1b[32m[ok]\x1b[39m No violation found (0ms).\n" || stderr.Len() != 0 {
		t.Fatalf("verify success status=%d out=%q err=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	stage := verifyStage{Status: "violation", Explanation: "Found an issue", Errors: []quint.ErrorMessage{{Explanation: "detail"}}, ViolatedInvariants: []string{"safe"}, Warnings: []string{"warning"}}
	if status := verifyOutputContext(ctx, verifyOptions{verbosity: 1}, stage, &stdout, &stderr); status != 1 || stdout.String() != "\x1b[31m❌ safe\x1b[39m\n" || stderr.String() != "\x1b[31mdetail\x1b[39m\n\x1b[31merror:\x1b[39m Found an issue\n\x1b[33m[warning] warning\x1b[39m\n" {
		t.Fatalf("verify failure status=%d out=%q err=%q", status, stdout.String(), stderr.String())
	}
}

func TestTerminalColoredOutputPreservesWriterFailure(t *testing.T) {
	t.Parallel()
	level := 1
	ctx := WithTerminalOptions(context.Background(), TerminalOptions{ColorLevel: &level})
	seed, err := quint.ParseSeed("17")
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("terminal output failed")
	writer := &executionFailWriter{failAt: 1, err: failure}
	var stderr bytes.Buffer
	if status := RunREPL(ctx, strings.NewReader("1\n2\n.exit\n"), writer, &stderr, REPLLoopOptions{Seed: &seed}); status != 1 {
		t.Fatalf("colored REPL ignored writer failure: status=%d stderr=%q", status, stderr.String())
	}
}

func TestTerminalREPLCommandStyles(t *testing.T) {
	t.Parallel()
	level := 1
	ctx := WithTerminalOptions(context.Background(), TerminalOptions{ColorLevel: &level})
	help := ".clear\tClear\n.load <filename> [module]\tLoad\n.seed[=<number>]\tSeed\n"
	want := "\x1b[31m.clear\x1b[39m\tClear\n\x1b[31m.load\x1b[39m <filename> \x1b[90m[module]\x1b[39m\tLoad\n\x1b[31m.seed\x1b[39m[=<number>]\tSeed\n"
	if got := terminalREPLHelp(ctx, help); got != want {
		t.Fatalf("help=%q want=%q", got, want)
	}
	if got := terminalREPLHelp(context.Background(), help+"\n.plain"); got != help+"\n.plain" {
		t.Fatalf("plain help changed: %q", got)
	}
	seed, err := quint.ParseSeed("17")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := RunREPL(ctx, strings.NewReader(".seed\n.verbosity=1\n.seed=17\n.save\n.exit\n"), &stdout, &stderr, REPLLoopOptions{Seed: &seed}); status != 0 || stderr.Len() != 0 {
		t.Fatalf("metadata status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	for _, token := range []string{"\x1b[90m.seed=17\x1b[39m", "\x1b[90m.verbosity=1\x1b[39m", "\x1b[31m.save requires a filename\x1b[39m"} {
		if !strings.Contains(stdout.String(), token) {
			t.Fatalf("missing %q in %q", token, stdout.String())
		}
	}
	var replay strings.Builder
	var writeErr error
	hooks := replReplayOutput(ctx, func() int { return 1 }, func(text string) { replay.WriteString(text) }, func(error) {}, func(quint.REPLResult) {}, &writeErr)
	if err := hooks.Before("history.qnt", "1"); err != nil || !strings.HasPrefix(replay.String(), "\x1b[90mEvaluating expression history in history.qnt\x1b[39m") {
		t.Fatalf("history header=%q error=%v", replay.String(), err)
	}
}
