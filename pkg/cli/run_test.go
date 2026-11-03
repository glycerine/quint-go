package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestRunOptions(t *testing.T) {
	t.Parallel()
	defaults, err := runParseOptions([]string{"counter.qnt"})
	if err != nil || defaults.maxSamples != 10000 || defaults.maxSteps != 20 || defaults.traces != 1 || defaults.verbosity != 2 || defaults.init != "init" || defaults.step != "step" || defaults.invariant != "true" {
		t.Fatal(defaults, err)
	}
	seeded, err := runParseOptions([]string{"--seed=0xff", "counter.qnt", "--mbt=false", "--no-mbt", "--invariants", "a", "b", "--witnesses=c", "--hide", "x", "y", "--backend=go", "--n-threads=1"})
	if err != nil || seeded.maxSamples != 1 || !seeded.seedSet || seeded.metadata || !reflect.DeepEqual(seeded.invariants, []string{"a", "b"}) || !reflect.DeepEqual(seeded.witnesses, []string{"c"}) || !reflect.DeepEqual(seeded.hide, []string{"x", "y"}) {
		t.Fatal(seeded, err)
	}
	explicit, err := runParseOptions([]string{"--seed", "", "--max-samples", "3", "--n-traces", "3", "--max-steps", "0", "--verbosity=0", "--mbt", "--", "counter.qnt"})
	if err != nil || explicit.maxSamples != 3 || explicit.traces != 3 || explicit.maxSteps != 0 || explicit.verbosity != 0 || !explicit.metadata {
		t.Fatal(explicit, err)
	}
	for _, args := range [][]string{
		{}, {"a.qnt", "b.qnt"}, {"a.qnt", "--unknown"}, {"a.qnt", "--backend=rust"},
		{"a.qnt", "--max-steps=-1"}, {"a.qnt", "--n-traces=no"}, {"a.qnt", "--verbosity=6"},
		{"a.qnt", "--no-mbt=false"}, {"a.qnt", "--n-threads=2"},
		{"a.qnt", "--seed=1", "--n-traces=2"}, {"--", "a.qnt", "b.qnt"},
	} {
		if _, err := runParseOptions(args); err == nil {
			t.Fatal("invalid arguments accepted", args)
		}
	}
}

func runTestSource(t *testing.T, source string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "different-filename.qnt")
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRunSourceExpressionsAndReplay(t *testing.T) {
	t.Parallel()
	file := runTestSource(t, "module Counter { var x:int action init=x'=0 action step=x'=x+1 val safe=x<3 }")
	outFile := filepath.Join(t.TempDir(), "out.json")
	var stdout, stderr bytes.Buffer
	args := []string{file, "--seed=42", "--max-steps=2", "--out", outFile, "--invariant", "safe and x >= 0", "--witnesses", "x == 2"}
	if status := commandRun(context.Background(), args, &stdout, &stderr); status != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	var stage struct {
		Stage, Status         string
		Seed                  quint.Integer
		Trace                 []quint.Expr
		Modules               []quint.Module
		Errors                []quint.ErrorMessage
		Types, Effects, Table json.RawMessage
	}
	if err = json.Unmarshal(data, &stage); err != nil {
		t.Fatal(err, string(data))
	}
	if stage.Stage != "running" || stage.Status != "ok" || stage.Seed.String() != "42" || len(stage.Trace) != 3 || len(stage.Modules) != 1 || len(stage.Errors) != 0 || len(stage.Types) < 3 || len(stage.Effects) < 3 || len(stage.Table) < 3 {
		t.Fatal(string(data))
	}
	if quint.ExpressionString(stage.Trace[2]) != "Rec(\"x\", 2)" {
		t.Fatal(quint.ExpressionString(stage.Trace[2]))
	}
	stdout.Reset()
	stderr.Reset()
	if status := commandRun(context.Background(), []string{file, "--seed=42", "--max-steps=2", "--verbosity=0"}, &stdout, &stderr); status != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
}

func TestRunViolationITFAndHiddenConsoleVariables(t *testing.T) {
	t.Parallel()
	file := runTestSource(t, "module Counter { var x:int action init=x'=0 action step=x'=x+1 val safe=x<2 }")
	dir := t.TempDir()
	template := filepath.Join(dir, "trace-{seq}.itf.json")
	var stdout, stderr bytes.Buffer
	args := []string{file, "--seed=7", "--max-samples=2", "--n-traces=1", "--max-steps=3", "--invariant=safe", "--out-itf", template, "--mbt", "--hide", "x", "--witnesses", "x == 1"}
	if status := commandRun(context.Background(), args, &stdout, &stderr); status != 1 || !strings.Contains(stdout.String(), "[violation] Found an issue") || !strings.Contains(stderr.String(), "Invariant violated") || !strings.Contains(stdout.String(), "was witnessed in 1 trace(s)") {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), `"x"`) {
		t.Fatal("hidden variable printed", stdout.String())
	}
	if strings.Contains(stdout.String(), "Use --seed") {
		t.Fatal("ITF output unexpectedly printed replay hint", stdout.String())
	}
	for i := 0; i < 1; i++ {
		data, err := os.ReadFile(quint.ExpandOutputTemplate(template, i, true))
		if err != nil {
			t.Fatal(err)
		}
		trace, err := quint.DecodeITFTrace(data)
		if err != nil || len(trace.States) != 3 || !bytes.Contains(trace.Metadata, []byte(`"status":"violation"`)) || !reflect.DeepEqual(trace.Vars, []string{"x", quint.ITFActionTaken, quint.ITFNondetPicks}) {
			t.Fatal(string(data), err)
		}
		if !bytes.Contains(data, []byte(`"init"`)) || bytes.Contains(data, []byte(`"q::init"`)) {
			t.Fatal("synthetic wrapper intercepted action metadata", string(data))
		}
		if !bytes.Contains(data, []byte(`"x"`)) {
			t.Fatal("console --hide leaked into ITF")
		}
	}
}

func TestRunImportsAndExplicitSelectors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := "module Counter { import Helper.* from \"helper\" var x:int action start=x'=zero action advance=x'=x+1 } module Other { val x=1 }"
	file := filepath.Join(dir, "counter.qnt")
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.qnt"), []byte("module Helper { pure val zero=0 }"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := commandRun(context.Background(), []string{file, "--main=Counter", "--init=start", "--step=advance", "--seed=0x123", "--max-steps=0", "--verbosity=1"}, &stdout, &stderr); status != 0 || !strings.Contains(stdout.String(), "--seed=0x123 --backend=go") || stderr.Len() != 0 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
}

func TestRunPreparationRuntimeAndOutputErrors(t *testing.T) {
	t.Parallel()
	file := runTestSource(t, "module Counter { var x:int action init=x'=0 action step=x'=x+1 val wrong=1 val boom=assert(false) }")
	for _, test := range []struct {
		args    []string
		message string
	}{
		{[]string{file, "--main=Missing"}, "QNT405"},
		{[]string{file, "--init=wrong"}, "default --init action name 'wrong' is used by a val declaration"},
		{[]string{file, "--init=42 +"}, "valid expression"},
		{[]string{file, "--invariant=missing"}, "not found"},
		{[]string{file, "--invariant=boom"}, "Assertion failed"},
		{[]string{file, "--seed=invalid"}, "--seed must be a big integer"},
		{[]string{file, "--out", filepath.Join(t.TempDir(), "missing", "out.json")}, "no such file"},
		{[]string{file, "--out-itf", filepath.Join(t.TempDir(), "missing", "trace.itf")}, "writing ITF trace"},
	} {
		var stdout, stderr bytes.Buffer
		args := append(append([]string{}, test.args...), "--seed=1", "--max-samples=1", "--max-steps=0", "--verbosity=0")
		if strings.Contains(test.message, "big integer") {
			args = test.args
		}
		if status := commandRun(context.Background(), args, &stdout, &stderr); status != 1 || !strings.Contains(stderr.String(), test.message) {
			t.Fatal(test.args, status, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if status := commandRun(canceled, []string{file}, &stdout, &stderr); status != 1 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatal(status, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := commandRun(context.Background(), []string{"--help"}, &stdout, &stderr); status != 0 || !strings.Contains(stdout.String(), "Usage: quint run") {
		t.Fatal(status, stdout.String(), stderr.String())
	}
}

func TestRunStateProjectionOwnership(t *testing.T) {
	t.Parallel()
	state, err := quint.ParseExpression(`Rec("x",1,"y",2)`)
	if err != nil {
		t.Fatal(err)
	}
	before := quint.ExpressionString(state)
	var output bytes.Buffer
	if err := runPrintTrace(&output, []quint.Expr{state}, []string{"x"}); err != nil || output.String() != "[State 0] { y: 2 }\n\n" || quint.ExpressionString(state) != before {
		t.Fatal(output.String(), quint.ExpressionString(state), err)
	}
	if got := runVariables([]quint.Expr{state}, false); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatal(got)
	}
}

func TestRunMultipleTracesAndAssignmentSelectorMetadata(t *testing.T) {
	t.Parallel()
	file := runTestSource(t, "module Counter { var x:int }")
	template := filepath.Join(t.TempDir(), "trace-{seq}.itf.json")
	var stdout, stderr bytes.Buffer
	args := []string{file, "--seed=3", "--max-samples=2", "--n-traces=2", "--max-steps=1", "--init", "x'=0", "--step", "x'=x+1", "--out-itf", template, "--mbt", "--verbosity=0"}
	if status := commandRun(context.Background(), args, &stdout, &stderr); status != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	for i := 0; i < 2; i++ {
		data, err := os.ReadFile(quint.ExpandOutputTemplate(template, i, true))
		if err != nil {
			t.Fatal(err)
		}
		trace, err := quint.DecodeITFTrace(data)
		if err != nil || len(trace.States) != 2 || !bytes.Contains(trace.Metadata, []byte(`"status":"ok"`)) {
			t.Fatal(string(data), err)
		}
		if bytes.Contains(data, []byte(`"q::init"`)) || bytes.Contains(data, []byte(`"q::step"`)) {
			t.Fatal("expression selectors introduced action metadata", string(data))
		}
	}
	state, err := quint.ParseExpression(`Rec("mbt::actionTaken", 1)`)
	if err != nil {
		t.Fatal(err)
	}
	if got := runVariables([]quint.Expr{state}, false); !reflect.DeepEqual(got, []string{quint.ITFActionTaken}) {
		t.Fatal("user variable interpreted as metadata", got)
	}
}
