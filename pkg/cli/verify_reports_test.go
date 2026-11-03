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
	"time"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

const verifyReportSource = `module M {
var x:int
action init=x'=0
action step=x'=x+1
val nonnegative=x>=0
val upper=x<3
}`

var verifyReportLabels = []string{"nonnegative", "upper", "x != 3"}

const verifyReportTrace = `{"vars":["x"],"states":[{"x":{"#bigint":"0"}},{"x":{"#bigint":"3"}}]}`

func verifyReportCompilation(t *testing.T, labels []string, reports bool) quint.CompilationResult {
	t.Helper()
	parsed, err := quint.ParseModulesWithLocations("M.qnt", verifyReportSource)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := quint.ResolveLinkedModules(parsed.Modules)
	if err != nil || len(linked.Errors) > 0 {
		t.Fatal(err, linked.Errors)
	}
	sorted, diagnostics := quint.SortModuleDeclarations(parsed.Modules, linked.Table)
	if len(diagnostics) > 0 {
		t.Fatal(diagnostics)
	}
	analyzed, err := quint.AnalyzeModules(sorted, linked.Table)
	if err != nil || len(analyzed.Diagnostics) > 0 {
		t.Fatal(err, analyzed.Diagnostics)
	}
	compiled, err := quint.CompileModules(analyzed.Modules, quint.AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes}, parsed.IDs, parsed.Sources, quint.CompilationOptions{Main: "M", CompilationEntries: quint.CompilationEntries{Target: "json", Flatten: true, Invariants: labels, InvariantReports: reports}})
	if err != nil || len(compiled.Diagnostics) > 0 || len(compiled.AnalysisDiagnostics) > 0 {
		t.Fatal(err, compiled.Diagnostics, compiled.AnalysisDiagnostics)
	}
	return compiled
}

func TestVerifyReportsOrdinaryInvariantsAtFinalState(t *testing.T) {
	t.Parallel()
	compiled := verifyReportCompilation(t, verifyReportLabels, true)
	stage := verifyStage{Status: "violation", Outcome: apalache.CheckViolation, Explanation: "found a counterexample", Trace: json.RawMessage(verifyReportTrace), ViolatedInvariants: []string{"previous"}, Warnings: []string{"existing warning"}}
	verifyReportInvariants(context.Background(), compiled, verifyReportLabels, &stage)
	if !reflect.DeepEqual(stage.ViolatedInvariants, []string{"previous", "upper", "x != 3"}) || !reflect.DeepEqual(stage.Warnings, []string{"existing warning"}) {
		t.Fatal(stage)
	}
	if stage.Status != "violation" || stage.Outcome != apalache.CheckViolation || stage.Explanation != "found a counterexample" || string(stage.Trace) != verifyReportTrace {
		t.Fatal("reporting changed the checker result", stage)
	}
	// A temporal or inductive failure need not refute an ordinary predicate at
	// the final state. Do not invent an ordinary failure from the verdict alone.
	stage = verifyStage{Status: "violation", Outcome: apalache.CheckViolation, Trace: json.RawMessage(`{"states":[{"x":{"#bigint":"0"}}]}`)}
	verifyReportInvariants(context.Background(), compiled, verifyReportLabels, &stage)
	if len(stage.ViolatedInvariants) > 0 || len(stage.Warnings) > 0 {
		t.Fatal("attributed an ordinary invariant that holds", stage)
	}
}

func TestVerifyReportsSkipOtherOutcomes(t *testing.T) {
	t.Parallel()
	verifyReportInvariants(context.Background(), quint.CompilationResult{}, verifyReportLabels, nil)
	for _, outcome := range []apalache.CheckOutcome{apalache.CheckSafe, apalache.CheckDeadlock, ""} {
		stage := verifyStage{Outcome: outcome, Trace: json.RawMessage(`{`)}
		verifyReportInvariants(context.Background(), quint.CompilationResult{}, verifyReportLabels, &stage)
		if len(stage.Warnings) > 0 || len(stage.ViolatedInvariants) > 0 || stage.Outcome != outcome {
			t.Fatal(stage)
		}
	}
	stage := verifyStage{Outcome: apalache.CheckViolation}
	verifyReportInvariants(context.Background(), quint.CompilationResult{}, nil, &stage)
	if len(stage.Warnings) > 0 || len(stage.ViolatedInvariants) > 0 {
		t.Fatal(stage)
	}
}

func TestVerifyReportsFailuresRemainWarnings(t *testing.T) {
	t.Parallel()
	compiled := verifyReportCompilation(t, verifyReportLabels, true)
	withoutReports := verifyReportCompilation(t, verifyReportLabels, false)
	for _, tc := range []struct {
		name    string
		trace   string
		missing bool
		cancel  bool
	}{
		{"missing trace", "", false, false},
		{"malformed trace", `{`, false, false},
		{"empty trace", `{"states":[]}`, false, false},
		{"invalid state", `{"states":[1]}`, false, false},
		{"unusable value", `{"states":[{"x":{"#unserializable":"Int"}}]}`, false, false},
		{"missing compiled predicates", verifyReportTrace, true, false},
		{"cancelled reporting", verifyReportTrace, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			input := compiled
			if tc.missing {
				input = withoutReports
			}
			stage := verifyStage{Status: "violation", Outcome: apalache.CheckViolation, Explanation: "found a counterexample", Trace: json.RawMessage(tc.trace), ViolatedInvariants: []string{"previous"}}
			verifyReportInvariants(ctx, input, verifyReportLabels, &stage)
			if len(stage.Warnings) != 1 || !strings.Contains(stage.Warnings[0], "could not identify violated invariants") || !reflect.DeepEqual(stage.ViolatedInvariants, []string{"previous"}) {
				t.Fatal(stage)
			}
			if stage.Status != "violation" || stage.Outcome != apalache.CheckViolation || stage.Explanation != "found a counterexample" || string(stage.Trace) != tc.trace {
				t.Fatal("reporting failure changed the checker result", stage)
			}
			var stdout, stderr bytes.Buffer
			if code := verifyOutput(verifyOptions{verbosity: 0}, stage, &stdout, &stderr); code != 1 || stdout.Len() > 0 || stderr.Len() > 0 {
				t.Fatal(code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestVerifyReportsCLIFromEitherChecker(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyReportSource)
	for _, name := range []string{"apalache-go", "tlc-go"} {
		t.Run(name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "result.json")
			called := false
			checkCompiled := func(compiled quint.CompilationResult) {
				called = true
				for i := range verifyReportLabels {
					found := false
					for _, declaration := range compiled.MainModule.Declarations {
						found = found || declaration.Name == quint.InvariantReportName(i)
					}
					if !found {
						t.Fatal("CLI did not retain separately compiled invariant", i)
					}
				}
			}
			backend := verifyTestBackend(func(_ context.Context, compiled quint.CompilationResult, _ json.RawMessage, _ apalache.VerificationOptions) (apalache.VerificationResult, error) {
				checkCompiled(compiled)
				return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckViolation, Counterexamples: []json.RawMessage{json.RawMessage(verifyReportTrace)}}}, nil
			})
			tlcBackend := verifyTLCBackend(func(_ context.Context, compiled quint.CompilationResult, options gotlc.Options) (gotlc.Result, error) {
				checkCompiled(compiled)
				if !options.ITF {
					t.Fatal("TLC invariant reporting did not request typed ITF")
				}
				return gotlc.Result{Outcome: gotlc.Violation, Complete: true, ITF: json.RawMessage(verifyReportTrace)}, nil
			})
			args := []string{input, "--backend=" + name, "--out=" + output, "--invariants=nonnegative", "--invariants=upper", "--invariants=x != 3"}
			var stdout, stderr bytes.Buffer
			code := verifyWithCheckers(context.Background(), args, &stdout, &stderr, backend, tlcBackend)
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var stage verifyStage
			if err := json.Unmarshal(data, &stage); err != nil || !called || code != 1 || stage.Status != "violation" || stage.Outcome != "" || len(stage.ViolatedInvariants) > 0 || len(stage.Warnings) > 0 || stdout.Len() > 0 || stderr.Len() > 0 {
				t.Fatal(code, string(data), stdout.String(), stderr.String(), err)
			}
			visible := []string{input, "--backend=" + name, "--verbosity=1", "--invariants=nonnegative", "--invariants=upper", "--invariants=x != 3"}
			if code := verifyWithCheckers(context.Background(), visible, &stdout, &stderr, backend, tlcBackend); code != 1 || !strings.Contains(stdout.String(), "❌ upper") || !strings.Contains(stdout.String(), "❌ x != 3") || strings.Contains(stdout.String(), "❌ nonnegative") {
				t.Fatal(code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestVerifyReportsTLCCannotConvertTrace(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyReportSource)
	output := filepath.Join(t.TempDir(), "result.json")
	called := false
	checker := verifyTLCBackend(func(_ context.Context, _ quint.CompilationResult, options gotlc.Options) (gotlc.Result, error) {
		called = true
		if !options.ITF {
			t.Fatal("TLC reporting did not request typed ITF")
		}
		return gotlc.Result{Outcome: gotlc.Violation, Complete: true, ITFError: "conversion unavailable"}, nil
	})
	var stdout, stderr bytes.Buffer
	code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc-go", "--invariants=nonnegative", "--invariants=upper", "--out=" + output}, &stdout, &stderr, nil, checker)
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var stage verifyStage
	if err := json.Unmarshal(data, &stage); err != nil || !called || code != 1 || stage.Status != "violation" || stage.Outcome != "" || len(stage.Trace) > 0 || len(stage.ViolatedInvariants) > 0 || len(stage.Warnings) > 0 || stdout.Len() > 0 || stderr.Len() > 0 {
		t.Fatal(code, string(data), stdout.String(), stderr.String(), err)
	}
	if code := verifyWithCheckers(context.Background(), []string{input, "--backend=tlc-go", "--verbosity=1", "--invariants=nonnegative", "--invariants=upper"}, &stdout, &stderr, nil, checker); code != 1 || !strings.Contains(stderr.String(), "could not identify violated invariants: conversion unavailable") {
		t.Fatal(code, stdout.String(), stderr.String())
	}
}

func TestVerifyReportsInductiveSingleOrdinaryInvariant(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyReportSource)
	output := filepath.Join(t.TempDir(), "result.json")
	called := false
	backend := verifyTestBackend(func(_ context.Context, compiled quint.CompilationResult, _ json.RawMessage, options apalache.VerificationOptions) (apalache.VerificationResult, error) {
		called = true
		if !options.Inductive {
			t.Fatal("induction was not requested")
		}
		found := false
		for _, declaration := range compiled.MainModule.Declarations {
			found = found || declaration.Name == quint.InvariantReportName(0)
		}
		if !found {
			t.Fatal("induction with one ordinary invariant omitted its report predicate")
		}
		return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckViolation, Counterexamples: []json.RawMessage{json.RawMessage(verifyReportTrace)}}, Phases: []apalache.VerificationPhaseResult{{Phase: apalache.ImplicationPhase}}}, nil
	})
	var stdout, stderr bytes.Buffer
	code := verifyWithBackend(context.Background(), []string{input, "--inductive-invariant=nonnegative", "--invariant=upper", "--out=" + output}, &stdout, &stderr, backend)
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var stage verifyStage
	if err := json.Unmarshal(data, &stage); err != nil || !called || code != 1 || stage.Status != "violation" || stage.Outcome != "" || len(stage.ViolatedInvariants) > 0 || len(stage.Warnings) > 0 || stdout.Len() > 0 || stderr.Len() > 0 {
		t.Fatal(code, string(data), stdout.String(), stderr.String(), err)
	}
	if code := verifyWithBackend(context.Background(), []string{input, "--inductive-invariant=nonnegative", "--invariant=upper", "--verbosity=1"}, &stdout, &stderr, backend); code != 1 || !strings.Contains(stdout.String(), "❌ upper") || strings.Contains(stdout.String(), "❌ nonnegative") {
		t.Fatal(code, stdout.String(), stderr.String())
	}
}

func TestVerifyReportsVerbosity(t *testing.T) {
	t.Parallel()
	stage := verifyStage{Status: "violation", Outcome: apalache.CheckViolation, Explanation: "found a counterexample", ViolatedInvariants: []string{"upper", "x != 3"}, Warnings: []string{"could not identify violated invariants: unavailable predicate"}}
	for _, verbosity := range []int{0, 1} {
		var stdout, stderr bytes.Buffer
		if code := verifyOutput(verifyOptions{verbosity: verbosity}, stage, &stdout, &stderr); code != 1 {
			t.Fatal(code, stdout.String(), stderr.String())
		}
		if verbosity == 0 && (stdout.Len() > 0 || stderr.Len() > 0) {
			t.Fatal("quiet mode printed a report", stdout.String(), stderr.String())
		}
		if verbosity == 1 && (!strings.Contains(stdout.String(), "❌ upper") || !strings.Contains(stdout.String(), "❌ x != 3") || !strings.Contains(stderr.String(), stage.Warnings[0]) || !strings.Contains(stderr.String(), stage.Explanation)) {
			t.Fatal("visible mode omitted invariant reporting", stdout.String(), stderr.String())
		}
	}
}

func TestVerifyReportsActualCheckersCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("actual CLI invariant-report checker corpus; set QUINT_CORPUS_TESTS=1")
	}
	input := verifyTestSource(t, `module M {
var x:bool
action init=x'=false
action step=x'=not(x)
val good=x or not(x)
val bad=not(x)
}`)
	for _, backend := range []string{"apalache-go", "tlc-go"} {
		t.Run(backend, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			directory := t.TempDir()
			output, itf := filepath.Join(directory, "result.json"), filepath.Join(directory, "trace.itf.json")
			args := []string{input, "--backend=" + backend, "--invariants=good", "--invariants=bad", "--out=" + output, "--out-itf=" + itf}
			if backend == "apalache-go" {
				args = append(args, "--max-steps=1")
			}
			var stdout, stderr bytes.Buffer
			code := commandVerify(ctx, args, &stdout, &stderr)
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var stage verifyStage
			if err := json.Unmarshal(data, &stage); err != nil || code != 1 || stage.Status != "violation" || stage.Outcome != "" || len(stage.ViolatedInvariants) > 0 || len(stage.Warnings) > 0 || len(stage.Errors) > 0 || stdout.Len() > 0 || stderr.Len() > 0 {
				t.Fatal(code, string(data), stdout.String(), stderr.String(), err)
			}
			raw, err := os.ReadFile(itf)
			if err != nil {
				t.Fatal(err)
			}
			trace, err := quint.DecodeITFTrace(raw)
			if err != nil || !reflect.DeepEqual(trace.Vars, []string{"x"}) || len(trace.States) < 2 {
				t.Fatal(string(stage.Trace), err)
			}
			previous := true
			for index, state := range trace.States {
				if state.Kind != "app" || state.Opcode != "Rec" || len(state.Args) != 2 || state.Args[0].Str != "x" || state.Args[1].Kind != "bool" {
					t.Fatal("counterexample is missing its typed Boolean state", state)
				}
				value := state.Args[1].Bool
				if value == previous {
					t.Fatal("counterexample violates init or the toggle transition", index, string(stage.Trace))
				}
				previous = value
			}
			if !previous {
				t.Fatal("final state does not refute bad", string(stage.Trace))
			}
		})
	}
}
