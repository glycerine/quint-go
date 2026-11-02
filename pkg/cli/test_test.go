package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func testCommandFile(t *testing.T, source string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "different_filename.qnt")
	if err := os.WriteFile(filename, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func testCommandCall(t *testing.T, ctx context.Context, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	status := commandTest(ctx, args, &stdout, &stderr)
	return status, stdout.String(), stderr.String()
}

func testCommandStage(t *testing.T, filename string) struct {
	Stage   string
	Passed  []string
	Failed  []string
	Ignored []string
	Errors  []struct{ Explanation string }
} {
	t.Helper()
	var stage struct {
		Stage   string
		Passed  []string
		Failed  []string
		Ignored []string
		Errors  []struct{ Explanation string }
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &stage); err != nil {
		t.Fatal(err, string(data))
	}
	return stage
}

func TestTestCommandResults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, match string
		status              int
		passed, failed      []string
		ignored             []string
	}{
		{"success and ignored", "module M { val firstTest=true val ignoredTest=7 val helper=false }", "", 0, []string{"firstTest"}, []string{}, []string{"ignoredTest"}},
		{"failure", "module M { val firstTest=false val secondTest=true }", "", 1, []string{"secondTest"}, []string{"firstTest"}, []string{}},
		{"regexp selection", "module M { val helper=false val skippedTest=true }", "^helper$", 1, []string{}, []string{"helper"}, []string{}},
		{"hidden import", "module A { val importedTest=true } module M { import A as I }", "", 0, []string{"I::importedTest"}, []string{}, []string{}},
		{"instance", "module A { const N:int val importedTest=N==1 } module M { import A(N=1) as I }", "", 0, []string{"M::I::importedTest"}, []string{}, []string{}},
		{"independent instances", "module A { const N:int val importedTest=N==1 } module M { import A(N=1) as I import A(N=2) as J }", "", 1, []string{"M::I::importedTest"}, []string{"M::J::importedTest"}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := testCommandFile(t, tc.source)
			output := filepath.Join(t.TempDir(), "stage.json")
			args := []string{input, "--main", "M", "--seed", "0", "--out", output}
			if tc.match != "" {
				args = append(args, "--match", tc.match)
			}
			status, stdout, stderr := testCommandCall(t, context.Background(), args...)
			if status != tc.status || stdout != "" || stderr != "" {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
			}
			stage := testCommandStage(t, output)
			if stage.Stage != "testing" || !slices.Equal(stage.Passed, tc.passed) || !slices.Equal(stage.Failed, tc.failed) || !slices.Equal(stage.Ignored, tc.ignored) {
				t.Fatalf("stage=%+v", stage)
			}
			if stage.Passed == nil || stage.Failed == nil || stage.Ignored == nil {
				t.Fatal("test result arrays must be present, including empty arrays")
			}
		})
	}
}

func TestTestCommandImportedReplayHint(t *testing.T) {
	t.Parallel()
	input := testCommandFile(t, "module A { val failureTest=false } module M { import A as I }")
	status, stdout, stderr := testCommandCall(t, context.Background(), input, "--main=M", "--seed=0")
	if status != 1 || !strings.Contains(stdout, "I::failureTest") || !strings.Contains(stdout, "--match=^failureTest$") || !strings.Contains(stderr, "Tests failed") {
		t.Fatal(status, stdout, stderr)
	}
}

func TestTestCommandSummaryAndEmptySelection(t *testing.T) {
	t.Parallel()
	input := testCommandFile(t, "module M { val firstTest=true val ignoredTest=7 val helper=false }")
	status, stdout, stderr := testCommandCall(t, context.Background(), "--seed=0", input)
	if status != 0 || stderr != "" || !strings.Contains(stdout, "firstTest") || !strings.Contains(stdout, "1 passing") || !strings.Contains(stdout, "1 ignored") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, stdout, stderr = testCommandCall(t, context.Background(), input, "--match=^helper$", "--seed=0")
	if status == 0 || !strings.Contains(stdout, "helper") || !strings.Contains(stdout+stderr, "Tests failed") || strings.Contains(stdout+stderr, "q::cliTest") {
		t.Fatalf("failed test leaked synthetic entry: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	// Empty --match preserves the reference default suffix selection.
	status, stdout, stderr = testCommandCall(t, context.Background(), input, "--match=", "--verbosity=0", "--seed=0")
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("empty match status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, _, stderr = testCommandCall(t, context.Background(), input, "--match", "^absent$", "--seed=0")
	if status == 0 || !strings.Contains(stderr, "no tests matched --match '^absent$'") {
		t.Fatal(status, stderr)
	}
	input = testCommandFile(t, "module M { val helper=true }")
	output := filepath.Join(t.TempDir(), "empty.json")
	status, stdout, stderr = testCommandCall(t, context.Background(), input, "--out", output)
	if status == 0 || stdout != "" || stderr != "" {
		t.Fatal(status, stdout, stderr)
	}
	stage := testCommandStage(t, output)
	if len(stage.Errors) != 1 || !strings.Contains(stage.Errors[0].Explanation, "no tests matched") {
		t.Fatalf("empty stage=%+v", stage)
	}
}

func TestTestCommandFileImport(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	input := filepath.Join(directory, "different_filename.qnt")
	for _, file := range []struct{ name, source string }{
		{"different_filename.qnt", `module M { import A as I from "library" }`},
		{"library.qnt", `module A { val importedTest=true }`},
	} {
		if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(directory, "stage.json")
	status, stdout, stderr := testCommandCall(t, context.Background(), input, "--seed=0", "--out", output)
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatal(status, stdout, stderr)
	}
	stage := testCommandStage(t, output)
	if !slices.Equal(stage.Passed, []string{"I::importedTest"}) {
		t.Fatalf("imported tests not selected: %+v", stage)
	}
}

func TestTestCommandZeroSamplesAndValidation(t *testing.T) {
	t.Parallel()
	input := testCommandFile(t, "module M { val failingTest=false }")
	status, stdout, stderr := testCommandCall(t, context.Background(), input, "--max-samples=0", "--verbosity=0")
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatal(status, stdout, stderr)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "input"},
		{[]string{input, "extra.qnt"}, "input"},
		{[]string{input, "--unknown"}, "unknown"},
		{[]string{input, "--main"}, "main"},
		{[]string{input, "--main", "--seed=0"}, "--main requires"},
		{[]string{input, "--out", "--seed=0"}, "--out requires"},
		{[]string{input, "--seed", "--verbosity=0"}, "--seed requires"},
		{[]string{input, "--main=Missing"}, "Missing"},
		{[]string{input, "--max-samples=-1"}, "max-samples"},
		{[]string{input, "--max-samples=1.5"}, "max-samples"},
		{[]string{input, "--verbosity=6"}, "verbosity"},
		{[]string{input, "--seed=nope"}, "seed"},
		{[]string{input, "--seed=0", "--seed=1"}, "seed"},
		{[]string{input, "--out-itf=first.json", "--output=second.json"}, "out-itf"},
		{[]string{input, "--backend=scala"}, "backend"},
		{[]string{input, "--match=["}, "match"},
		{[]string{input, "--match=(?=failing)"}, "match"},
	} {
		status, _, stderr := testCommandCall(t, context.Background(), tc.args...)
		if status == 0 || !strings.Contains(strings.ToLower(stderr), strings.ToLower(tc.want)) {
			t.Fatalf("%v status=%d stderr=%q", tc.args, status, stderr)
		}
	}
	status, stdout, stderr = testCommandCall(t, context.Background(), "--help")
	if status != 0 || !strings.Contains(stdout, "test") || stderr != "" {
		t.Fatal(status, stdout, stderr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	status, _, stderr = testCommandCall(t, ctx, input)
	if status == 0 || !strings.Contains(stderr, "canceled") {
		t.Fatal(status, stderr)
	}
}

func TestTestCommandStaticAndFileErrors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"module M { val invalidTest=1+true }", "module M { val invalidTest=missing }", "module M { val invalidTest= }"} {
		input := testCommandFile(t, source)
		status, _, stderr := testCommandCall(t, context.Background(), input, "--seed=0")
		if status == 0 || stderr == "" {
			t.Fatal(source, status, stderr)
		}
	}
	input := filepath.Join(t.TempDir(), "missing.qnt")
	status, _, stderr := testCommandCall(t, context.Background(), input)
	if status == 0 || !strings.Contains(stderr, "missing.qnt") {
		t.Fatal(status, stderr)
	}
	input = testCommandFile(t, "module M { val firstTest=true }")
	status, _, stderr = testCommandCall(t, context.Background(), input, "--out", filepath.Join(t.TempDir(), "absent", "out.json"))
	if status == 0 || stderr == "" {
		t.Fatal(status, stderr)
	}
}

func TestTestCommandITF(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		status       int
		writes       bool
	}{
		{"stateTest", "module M { var x:int action stateTest=x'=123456789012345678901234567890 }", 0, true},
		{"falseTest", "module M { val falseTest=false }", 1, true},
		{"ignoredTest", "module M { val ignoredTest=7 }", 0, false},
		{"crashTest", "module M { val crashTest=assert(false) }", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := testCommandFile(t, tc.source)
			directory := t.TempDir()
			template := filepath.Join(directory, "{test}-{seq}.json")
			status, _, stderr := testCommandCall(t, context.Background(), input, "--seed=0", "--output", template, "--verbosity=0")
			if status != tc.status {
				t.Fatal(status, stderr)
			}
			filename := filepath.Join(directory, tc.name+"-0.json")
			data, err := os.ReadFile(filename)
			if !tc.writes {
				if !os.IsNotExist(err) {
					t.Fatal("ignored/runtime-error test wrote ITF", string(data), err)
				}
				return
			}
			if err != nil {
				t.Fatal(err, stderr)
			}
			var trace struct {
				Vars   []string
				States []json.RawMessage
				Meta   struct{ Status string } `json:"#meta"`
			}
			if err = json.Unmarshal(data, &trace); err != nil || len(trace.States) != 1 || trace.Meta.Status != map[bool]string{true: "failed", false: "passed"}[tc.status != 0] {
				t.Fatal(string(data), err)
			}
			if tc.name == "stateTest" && (!slices.Contains(trace.Vars, "x") || !bytes.Contains(data, []byte("123456789012345678901234567890"))) {
				t.Fatal("state or integer lost", string(data))
			}
		})
	}
}

func TestTestCommandOptionDefaultsAndMatcher(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args    []string
		input   string
		samples int
		seed    bool
	}{
		{[]string{"input.qnt"}, "input.qnt", 10000, false},
		{[]string{"--seed", "0x42", "input.qnt"}, "input.qnt", 1, true},
		{[]string{"input.qnt", "--seed=0", "--max-samples=7"}, "input.qnt", 7, true},
		{[]string{"--max-samples=0", "input.qnt", "--seed=0"}, "input.qnt", 0, true},
		{[]string{"--backend=go", "--", "-input.qnt"}, "-input.qnt", 10000, false},
	} {
		got, err := testParseOptions(tc.args)
		if err != nil || got.input != tc.input || got.maxSamples != tc.samples || got.seedSet != tc.seed {
			t.Fatalf("%v parsed=%+v err=%v", tc.args, got, err)
		}
	}
	for _, pattern := range []string{`\p{L}`, `\P{L}`, `(?=name)`, `(?!name)`, `(?<=name)`, `(?P<name>example)`, `(?i)name`, `(?m)name`, `(?s)name`, `(?U)name`, `(?-i)name`, `\Aname`, `name\z`, `\Qname\E`, `\a`, `\x{41}`, `[[:alpha:]]`, `(name)\1`} {
		if _, err := testMatcher(pattern); err == nil {
			t.Fatal("unsupported regex accepted", pattern)
		}
	}
	for _, pattern := range []string{"Test$", "^example", "(example|helper)[0-9]*"} {
		match, err := testMatcher(pattern)
		if err != nil || !match("exampleTest") || match("other") {
			t.Fatal(pattern, err)
		}
	}
	match, err := testMatcher("")
	if err != nil || match != nil {
		t.Fatal("empty matcher must use suffix default", err)
	}
}

type testCommandBrokenWriter struct{ err error }

func (w testCommandBrokenWriter) Write([]byte) (int, error) { return 0, w.err }

func TestTestCommandOutputErrors(t *testing.T) {
	t.Parallel()
	input := testCommandFile(t, "module M { val exampleTest=true }")
	failure := errors.New("test output destination unavailable")
	for _, tc := range []struct {
		args []string
		err  error
		want string
	}{
		{[]string{"--help"}, failure, failure.Error()},
		{[]string{input, "--seed=0"}, failure, failure.Error()},
		{[]string{"--help"}, nil, io.ErrShortWrite.Error()},
		{[]string{input, "--seed=0"}, nil, io.ErrShortWrite.Error()},
	} {
		var stderr bytes.Buffer
		status := commandTest(context.Background(), tc.args, testCommandBrokenWriter{tc.err}, &stderr)
		if status == 0 || !strings.Contains(stderr.String(), tc.want) {
			t.Fatal(tc.args, status, stderr.String())
		}
	}
	if status := commandTest(context.Background(), []string{"--help"}, nil, nil); status != 0 {
		t.Fatal("nil output should discard", status)
	}
}

func TestTestCommandInvalidSeedPrecedesOutput(t *testing.T) {
	t.Parallel()
	output := filepath.Join(t.TempDir(), "stage.json")
	status, stdout, stderr := testCommandCall(t, context.Background(), "missing.qnt", "--seed=NotANumber", "--out="+output)
	if status != 1 || stdout != "" || stderr != "--seed must be a big integer, found: NotANumber\n" {
		t.Fatal(status, stdout, stderr)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("argument coercion must precede loading and stage output", err)
	}
}
