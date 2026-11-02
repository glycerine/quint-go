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
	"github.com/glycerine/quint-go/pkg/backend/apalache"
)

type verifyTestBackend func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error)

type verifyBrokenWriter struct{}

func (verifyBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func (f verifyTestBackend) Verify(ctx context.Context, compiled quint.CompilationResult, loaded json.RawMessage, options apalache.VerificationOptions) (apalache.VerificationResult, error) {
	return f(ctx, compiled, loaded, options)
}

func verifyTestSource(t *testing.T, source string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "M.qnt")
	if err := os.WriteFile(name, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

const verifyTestModel = "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 val upper=x<3 temporal live=always(safe) }"

func TestVerifyCLICompileOptionsAndConfiguration(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	config := filepath.Join(t.TempDir(), "config.json")
	const configuration = `{"checker":{"length":999,"tuning":{"opaque":"9007199254740993"}}}`
	if err := os.WriteFile(config, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	backend := verifyTestBackend(func(ctx context.Context, compiled quint.CompilationResult, loaded json.RawMessage, options apalache.VerificationOptions) (apalache.VerificationResult, error) {
		called = true
		if options.MaxSteps != 7 || !options.RandomTransitions || !options.Inductive || options.Invariants != nil || len(options.Temporal) != 1 || options.Temporal[0] != "q::temporalProps" || string(loaded) != configuration {
			t.Fatal(options, string(loaded))
		}
		if compiled.Main != "M" || len(compiled.Modules) != 1 || compiled.MainModule.Name != "M" {
			t.Fatal("source compilation", compiled.Main)
		}
		wrappers := map[string]bool{}
		for _, d := range compiled.MainModule.Declarations {
			wrappers[d.Name] = true
			if d.Name == "q::inv" && !strings.Contains(quint.ExpressionString(*d.Expr), "and") {
				t.Fatal("invariants not combined")
			}
		}
		for _, name := range []string{"q::init", "q::step", "q::inv", "q::inductiveInv", "q::temporalProps"} {
			if !wrappers[name] {
				t.Fatal("wrapper missing", name)
			}
		}
		actualConfig, err := apalache.CheckConfig(compiled, loaded, options.CheckOptions)
		if err != nil || !strings.Contains(string(actualConfig), `"length":7`) || !strings.Contains(string(actualConfig), `"search.simulation":"true"`) {
			t.Fatal(string(actualConfig), err)
		}
		return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckSafe}, Phases: []apalache.VerificationPhaseResult{{Phase: apalache.InitializationPhase}, {Phase: apalache.PreservationPhase}, {Phase: apalache.ImplicationPhase}}}, nil
	})
	var stdout, stderr bytes.Buffer
	code := verifyWithBackend(context.Background(), []string{input, "--backend=apalache-go", "--max-steps", "7", "--invariant", "safe", "--invariants", "upper", "safe", "--inductive-invariant", "safe", "--temporal=live", "--random-transitions", "--apalache-config", config}, &stdout, &stderr, backend)
	if code != 0 || !called || !strings.Contains(stdout.String(), "Inductive invariant verified") || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String(), called)
	}
}

func TestVerifyCLIBoundedDefaultsAndNoInvariant(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	called := false
	backend := verifyTestBackend(func(_ context.Context, _ quint.CompilationResult, loaded json.RawMessage, options apalache.VerificationOptions) (apalache.VerificationResult, error) {
		called = true
		if options.MaxSteps != 10 || options.RandomTransitions || options.Inductive || options.Invariants == nil || len(options.Invariants) != 0 || loaded != nil || len(options.Temporal) != 0 {
			t.Fatal(options, loaded)
		}
		return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckSafe}}, nil
	})
	var stdout, stderr bytes.Buffer
	code := verifyWithBackend(context.Background(), []string{input}, &stdout, &stderr, backend)
	if code != 0 || !called || !strings.Contains(stdout.String(), "up to 10 steps") || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String(), called)
	}
}

func TestVerifyCLIDefaultMainUsesEntrySourceModules(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source, main string
		imported, relative bool
	}{
		{"single-module", "module Main { action init=true action step=true }", "Main", false, false},
		{"imported-module", "module Main { import Library.* from \"Library\" var x:int action init=x'=zero action step=x'=x+1 }", "Main", true, false},
		{"relative-input", "module Main { action init=true action step=true }", "Main", false, true},
		{"multiple-entry-modules", "module Other { pure val n=1 } module entry { action init=true action step=true }", "entry", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "entry.qnt")
			if err := os.WriteFile(input, []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.imported {
				if err := os.WriteFile(filepath.Join(dir, "Library.qnt"), []byte("module Library { pure val zero=0 }"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.relative {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				input, err = filepath.Rel(cwd, input)
				if err != nil {
					t.Fatal(err)
				}
			}
			called := false
			backend := verifyTestBackend(func(_ context.Context, compiled quint.CompilationResult, _ json.RawMessage, _ apalache.VerificationOptions) (apalache.VerificationResult, error) {
				called = true
				if compiled.Main != test.main || compiled.MainModule.Name != test.main {
					t.Fatal("incorrect main module", compiled.Main)
				}
				for _, declaration := range compiled.MainModule.Declarations {
					if declaration.Kind == "import" || declaration.Kind == "instance" {
						t.Fatal("main module was not flattened")
					}
				}
				return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckSafe}}, nil
			})
			var stdout, stderr bytes.Buffer
			if code := verifyWithBackend(context.Background(), []string{input}, &stdout, &stderr, backend); code != 0 || !called || stderr.Len() != 0 {
				t.Fatal(code, stdout.String(), stderr.String(), called)
			}
		})
	}
}

func TestVerifyCLIInvalidInvocation(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		nil, {"A.qnt", "B.qnt"}, {"A.qnt", "--max-steps=-1"}, {"A.qnt", "--max-steps=no"},
		{"A.qnt", "--verbosity=6"}, {"A.qnt", "--verbosity=-1"}, {"A.qnt", "--invariants"},
		{"A.qnt", "--backend=tlc"}, {"A.qnt", "--backend=scala"}, {"A.qnt", "--unknown=1"},
		{"A.qnt", "--server-endpoint=localhost:8822"}, {"A.qnt", "--apalache-version=1"}, {"A.qnt", "--tlc-config=config.json"},
		{"A.qnt", "--main"},
	} {
		var stdout, stderr bytes.Buffer
		code := verifyWithBackend(context.Background(), args, &stdout, &stderr, verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
			t.Fatal("backend invoked for invalid arguments", args)
			return apalache.VerificationResult{}, nil
		}))
		if code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatal(args, code, stdout.String(), stderr.String())
		}
	}
	for _, help := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		if code := commandVerify(context.Background(), []string{help}, &stdout, &stderr); code != 0 || strings.Count(stdout.String(), "Usage:") != 1 || stderr.Len() != 0 {
			t.Fatal(code, stdout.String(), stderr.String())
		}
	}
}

func TestVerifyCLIInterspersedBooleanAndArrayFlags(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args   []string
		random bool
	}{
		{[]string{"--main=M", "M.qnt", "--invariants=safe", "--random-transitions", "false"}, false},
		{[]string{"M.qnt", "--random-transitions", "--no-random-transitions", "--invariants", "safe"}, false},
		{[]string{"--invariants=safe", "--random-transitions", "true", "--", "-model.qnt"}, true},
	} {
		var stdout, stderr bytes.Buffer
		options, help, err := verifyParseOptions(test.args, &stdout, &stderr)
		if err != nil || help || options.randomTransitions != test.random || options.input == "" || len(options.invariants) != 1 || options.invariants[0] != "safe" || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatal(options, help, err, stdout.String(), stderr.String())
		}
	}
}

func TestVerifyCLIFrontendAndConfigurationFailures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	badConfig := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badConfig, []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		source string
		extra  []string
		want   string
	}{
		{"module M { val safe=missing }", nil, "parsing failed"},
		{"module M { val safe=1+true }", nil, "typechecking failed"},
		{"module M { val safe=1 }", nil, "compilation failed"},
		{verifyTestModel, []string{"--main=Missing"}, "does not exist"},
		{verifyTestModel, []string{"--invariant=missing"}, "compilation failed"},
		{verifyTestModel, []string{"--apalache-config", badConfig}, "must be a JSON object"},
		{verifyTestModel, []string{"--apalache-config", filepath.Join(dir, "missing")}, "load Apalache configuration"},
		{"module M {", nil, "invalid module"},
	}
	for _, test := range cases {
		input := verifyTestSource(t, test.source)
		var stdout, stderr bytes.Buffer
		code := verifyWithBackend(context.Background(), append([]string{input}, test.extra...), &stdout, &stderr, verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
			t.Fatal("backend invoked for invalid frontend", test)
			return apalache.VerificationResult{}, nil
		}))
		if code != 1 || stdout.Len() != 0 || !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(test.want)) {
			t.Fatal(test, code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := commandVerify(context.Background(), []string{filepath.Join(dir, "missing.qnt")}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "ENOENT") {
		t.Fatal(code, stderr.String())
	}
}

func TestVerifyCLIResultFilesAndBackendFailures(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	trace := json.RawMessage(`{"#meta":{"loop":0},"vars":["x"],"states":[{"x":{"#bigint":"9007199254740993"}}]}`)
	for _, test := range []struct {
		outcome apalache.CheckOutcome
		err     error
		status  string
		code    int
	}{
		{apalache.CheckSafe, nil, "ok", 0},
		{apalache.CheckViolation, nil, "violation", 1},
		{apalache.CheckDeadlock, nil, "violation", 1},
		{"", errors.New("solver unknown"), "failure", 1},
		{"invalid", nil, "failure", 1},
	} {
		dir := t.TempDir()
		out, itf := filepath.Join(dir, "out.json"), filepath.Join(dir, "trace.itf.json")
		backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
			return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: test.outcome, Counterexamples: []json.RawMessage{trace}}, Phases: []apalache.VerificationPhaseResult{{Phase: apalache.BoundedPhase}}}, test.err
		})
		var stdout, stderr bytes.Buffer
		code := verifyWithBackend(context.Background(), []string{input, "--out", out, "--out-itf", itf}, &stdout, &stderr, backend)
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var stage verifyStage
		if err = json.Unmarshal(data, &stage); err != nil || stage.Status != test.status || len(stage.Phases) != 1 || code != test.code || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatal(test, stage, code, stdout.String(), stderr.String(), err)
		}
		if test.outcome == apalache.CheckViolation || test.outcome == apalache.CheckDeadlock {
			data, err = os.ReadFile(itf)
			if err != nil || string(bytes.TrimSpace(data)) != string(trace) {
				t.Fatal("ITF precision or metadata lost", string(data), err)
			}
		} else if _, err = os.Stat(itf); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("wrote absent counterexample", err)
		}
	}
}

func TestVerifyCLICancellationQuietAndWriteErrors(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	if code := commandVerify(ctx, []string{input}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatal(code, stderr.String())
	}
	backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
		return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckViolation}}, nil
	})
	stdout.Reset()
	stderr.Reset()
	if code := verifyWithBackend(context.Background(), []string{input, "--verbosity=0"}, &stdout, &stderr, backend); code != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	stage := verifyStage{Status: "violation", Trace: json.RawMessage(`{"states":[]}`)}
	for _, options := range []verifyOptions{{out: t.TempDir()}, {outITF: t.TempDir()}} {
		stderr.Reset()
		if code := verifyOutput(options, stage, io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "write") {
			t.Fatal(code, stderr.String())
		}
	}
	stderr.Reset()
	stage.Trace = json.RawMessage(`{`)
	if code := verifyOutput(verifyOptions{outITF: filepath.Join(t.TempDir(), "trace.json")}, stage, io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "invalid ITF") {
		t.Fatal(code, stderr.String())
	}
	stderr.Reset()
	if code := verifyOutput(verifyOptions{verbosity: 2}, verifyStage{Status: "ok"}, verifyBrokenWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "write verification result") {
		t.Fatal(code, stderr.String())
	}
}

func TestVerifyCLIProductionGoBackend(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }")
	var stdout, stderr bytes.Buffer
	code := commandVerify(context.Background(), []string{input, "--invariant=safe", "--max-steps=0"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "No violation found up to 0 steps") || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
}
