package tlc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	quint "github.com/glycerine/quint-go/pkg"
	native "github.com/glycerine/tlago/tlc"
)

const booleanTLA = `---- MODULE M ----
VARIABLE x
q_init == x = FALSE
q_step == x' = ~x
q_inv == x \in BOOLEAN
Bad == ~x
====`

func TestNativeTLCSafety(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		opts         Options
		outcome      Outcome
	}{
		{"safe", booleanTLA, Options{Invariants: []string{"q_inv"}}, Safe},
		{"violation", booleanTLA, Options{Invariants: []string{"Bad"}}, Violation},
		{"multiple invariants", booleanTLA, Options{Workers: 2, Invariants: []string{"q_inv", "Bad"}}, Violation},
		{"initial violation", strings.Replace(booleanTLA, "x = FALSE", "x = TRUE", 1), Options{Invariants: []string{"Bad"}}, Violation},
		{"deadlock", strings.Replace(booleanTLA, "x' = ~x", "x /\\ x' = ~x", 1), Options{CheckDeadlock: true}, Deadlock},
		{"deadlock disabled", strings.Replace(booleanTLA, "x' = ~x", "x /\\ x' = ~x", 1), Options{}, Safe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tc.opts.Workers == 0 {
				tc.opts.Workers = 1
			}
			result, err := CheckTLA(ctx, "M.tla", tc.source, tc.opts)
			if err != nil || !result.Complete || result.Outcome != tc.outcome {
				t.Fatalf("%+v err=%v", result, err)
			}
			if result.InitialStates != 1 || result.DistinctStates < 1 {
				t.Fatal(result)
			}
			if tc.outcome == Violation || tc.outcome == Deadlock {
				if len(result.Trace) == 0 {
					t.Fatal("missing concrete counterexample", result)
				}
				for i, state := range result.Trace {
					if state.Index != i || len(state.Values) != 1 || state.Values[0].Name != "x" {
						t.Fatal(result.Trace)
					}
				}
				if tc.name == "violation" && (len(result.Trace) != 2 || result.Trace[0].Values[0].Value != "FALSE" || result.Trace[1].Values[0].Value != "TRUE") {
					t.Fatal(result.Trace)
				}
				if tc.name == "initial violation" && (len(result.Trace) != 1 || result.Trace[0].Values[0].Value != "TRUE") {
					t.Fatal(result.Trace)
				}
			} else if len(result.Trace) != 0 {
				t.Fatal(result.Trace)
			}
		})
	}
}

func TestConfigurationAndValidation(t *testing.T) {
	for _, input := range []string{"", `null`, `[]`, `{`, `{} {}`, `{"workers":-1}`, `{"workers":"many"}`, `{"workers":null}`, `{"workers":0}`, `{"workers":1.5}`, `{"maxHeap":"8G"}`, `{"stackSize":"512m"}`, `{"typo":true}`, `{"timeoutMillis":-1}`, `{"timeoutMillis":9223372036854775807}`, `{"timeoutMillis":null}`, `{"checkDeadlock":null}`, `{"checkDeadlock":1}`, `{"timeoutMillis":"2"}`} {
		if options, err := ParseConfig([]byte(input)); err == nil {
			t.Fatalf("accepted %s as %+v", input, options)
		}
	}
	options, err := ParseConfig([]byte(`{"workers":2,"checkDeadlock":true,"timeoutMillis":12}`))
	if err != nil || options.Workers != 2 || !options.CheckDeadlock || options.MaxDuration != 12*time.Millisecond {
		t.Fatal(options, err)
	}
	if options, err := ParseConfig([]byte(`{"workers":"auto"}`)); err != nil || options.Workers != 0 {
		t.Fatal(options, err)
	}
	if options, err := ParseConfig([]byte(`{}`)); err != nil || options.Workers != 0 {
		t.Fatal(options, err)
	}
	for _, options := range []Options{{Workers: -1}, {MaxDuration: -1}, {Temporal: []string{"bad::name"}}, {Temporal: []string{""}}, {Init: "Init\nNEXT Other"}, {Next: "a-b"}, {Invariants: []string{"bad::name"}}, {Invariants: []string{""}}} {
		if result, err := CheckTLA(context.Background(), "M.tla", booleanTLA, options); err == nil || result.Complete || result.Outcome != "" {
			t.Fatal(result, err)
		}
		if _, err := Check(context.Background(), quint.CompilationResult{}, options); err == nil {
			t.Fatal(options)
		}
	}
	for _, name := range []string{"A", "A12", "_ok", "a_b"} {
		if !validName(name) {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"", "12A", "a.b", "ümlaut"} {
		if validName(name) {
			t.Fatal(name)
		}
	}
}

func TestNativeTLCUnsafeInheritedModes(t *testing.T) {
	native.Globals.Lock()
	continuation, dfid := native.Globals.Continuation, native.Globals.DFIDMax
	workers, metadata, tool, start := native.Globals.NumWorkers, native.Globals.MetaDir, native.Globals.Tool, native.Globals.StartTime
	checker := native.Globals.MainChecker
	callerStart := time.Unix(1700000000, 1234)
	native.Globals.Continuation = true
	native.Globals.DFIDMax = 2
	native.Globals.NumWorkers = 7
	native.Globals.MetaDir = "caller/metadata"
	native.Globals.Tool = true
	native.Globals.StartTime = callerStart
	native.Globals.Unlock()
	defer func() {
		native.Globals.Lock()
		native.Globals.Continuation = continuation
		native.Globals.DFIDMax = dfid
		native.Globals.NumWorkers = workers
		native.Globals.MetaDir = metadata
		native.Globals.Tool = tool
		native.Globals.StartTime = start
		native.Globals.MainChecker = checker
		native.Globals.Unlock()
	}()
	source := `---- MODULE M ----
VARIABLE x,y
q_init == /\ x = FALSE
          /\ y = FALSE
q_step == /\ x' = ~y
          /\ y' = x
Bad == ~(~x /\ y)
====`
	result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, Invariants: []string{"Bad"}})
	if err != nil || result.Outcome != Violation || !result.Complete || len(result.Trace) != 4 {
		t.Fatal(result, err)
	}
	for _, state := range result.Trace {
		if len(state.Values) != 2 || state.Values[0].Name != "x" || state.Values[1].Name != "y" {
			t.Fatal(result.Trace)
		}
	}
	native.Globals.Lock()
	restored := native.Globals.Continuation
	restoredSettings := native.Globals.NumWorkers == 7 && native.Globals.DFIDMax == 2 && native.Globals.MetaDir == "caller/metadata" && native.Globals.Tool && native.Globals.StartTime == callerStart && native.Globals.MainChecker == checker
	native.Globals.Unlock()
	if !restored || !restoredSettings {
		t.Fatal("native caller runtime settings were not restored")
	}
	result, err = CheckTLA(context.Background(), "M.tla", "invalid TLA", Options{Workers: 1})
	if err == nil || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
	native.Globals.Lock()
	restoredSettings = native.Globals.Continuation && native.Globals.NumWorkers == 7 && native.Globals.DFIDMax == 2 && native.Globals.MetaDir == "caller/metadata" && native.Globals.Tool && native.Globals.StartTime == callerStart && native.Globals.MainChecker == checker
	native.Globals.Unlock()
	if !restoredSettings {
		t.Fatal("failed checker call changed caller runtime settings")
	}
}

func TestNativeTLCRejectsCachedProbabilisticMode(t *testing.T) {
	native.Globals.Lock()
	previous := native.Globals.Probabilistic
	native.Globals.Probabilistic = true
	native.Globals.Unlock()
	native.InitializeToolStatics()
	native.InitializeToolProperties()
	native.Globals.Lock()
	native.Globals.Probabilistic = previous
	native.Globals.Unlock()
	defer native.InitializeToolStatics()
	result, err := CheckTLA(context.Background(), "M.tla", booleanTLA, Options{Workers: 1})
	if err == nil || !strings.Contains(err.Error(), "probabilistic") || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
}

func TestPreparationAndCheckerFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := CheckTLA(ctx, "M.tla", booleanTLA, Options{}); !errors.Is(err, context.Canceled) || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
	if _, err := Check(ctx, quint.CompilationResult{}, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		source  string
		options Options
	}{
		{"bad source", Options{}},
		{booleanTLA, Options{Init: "missing"}},
		{booleanTLA, Options{Invariants: []string{"missing"}}},
		{strings.Replace(booleanTLA, "x = FALSE", "x = 2147483648", 1), Options{}},
		{strings.Replace(booleanTLA, "q_inv == x \\in BOOLEAN", "q_inv == x[FALSE]", 1), Options{Invariants: []string{"q_inv"}}},
	} {
		result, err := CheckTLA(context.Background(), "M.tla", tc.source, withOneWorker(tc.options))
		if err == nil || result.Complete || result.Outcome != "" {
			t.Fatal(result, err)
		}
	}
	if _, err := Check(context.Background(), quint.CompilationResult{}, Options{}); err == nil {
		t.Fatal("accepted empty compilation")
	}
	if trace := counterexample([]native.Message{{Code: native.ECTLCStatePrint1}, {Code: native.ECTLCStatePrint2}, {Code: native.ECTLCSuccess}}, []string{"x"}); len(trace) != 0 {
		t.Fatal(trace)
	}
}

func TestNativeTLCCustomOperatorsAndConcurrentCalls(t *testing.T) {
	source := strings.ReplaceAll(strings.ReplaceAll(booleanTLA, "q_init", "Start"), "q_step", "Advance")
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, Init: "Start", Next: "Advance", Invariants: []string{"q_inv"}})
			if err == nil && (result.Outcome != Safe || !result.Complete || result.DistinctStates != 2) {
				err = fmt.Errorf("unexpected result %+v", result)
			}
			errors <- err
		}()
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
}

func TestTemporaryDirectoryFailure(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	result, err := CheckTLA(context.Background(), "M.tla", booleanTLA, Options{})
	if err == nil || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
}

func TestOwnershipCancellation(t *testing.T) {
	ownership <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := CheckTLA(ctx, "M.tla", booleanTLA, Options{})
	<-ownership
	if !errors.Is(err, context.Canceled) || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
}

func TestNativeTLCCancellation(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC cancellation corpus; set QUINT_CORPUS_TESTS=1")
	}
	source := `---- MODULE M ----
EXTENDS Integers
VARIABLE x
q_init == x = 0
q_step == x' = x + 1
====`
	result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, MaxDuration: 200 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
	if result.StatesGenerated < 10 {
		t.Fatalf("cancellation did not reach running checker: %+v", result)
	}
	for _, tc := range []struct{ name, init, next string }{
		{"initial fanout", `x \in 1..100000`, `x'=x`},
		{"next fanout", `x=0`, `x' \in 1..100000`},
		{"unsatisfied initial fanout", `\E y \in 1..1000000 : y<0 /\ x=y`, `x'=x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Integers\nVARIABLE x\nq_init == %s\nq_step == %s\n====", tc.init, tc.next)
			start := time.Now()
			result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, MaxDuration: 100 * time.Millisecond})
			if !errors.Is(err, context.DeadlineExceeded) || result.Complete || result.Outcome != "" {
				t.Fatal(result, err)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("state enumeration ignored cancellation for %s", elapsed)
			}
			if tc.name == "initial fanout" && result.InitialStates >= 100000 {
				t.Fatal("initial enumeration was not interrupted", result.InitialStates)
			}
		})
	}
}

// Native tool construction expands this constant quantifier before callbacks
// can be installed. Its soft deadline must still never report safe/complete.
func TestNativeTLCPreparationDeadline(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC preparation cancellation corpus; set QUINT_CORPUS_TESTS=1")
	}
	source := `---- MODULE M ----
EXTENDS Integers
VARIABLE x
q_init == x=0
q_step == \E y \in 1..1000000 : y<0 /\ x'=y
====`
	result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, MaxDuration: 100 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
}

func TestNativeTLCExpiredDeadlineCannotSucceed(t *testing.T) {
	result, err := CheckTLA(context.Background(), "M.tla", booleanTLA, Options{Workers: 1, MaxDuration: time.Nanosecond})
	if !errors.Is(err, context.DeadlineExceeded) || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
}

func compileSource(t *testing.T, source string, temporal ...string) quint.CompilationResult {
	t.Helper()
	parsed, err := quint.ParseModulesWithLocations("M.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := quint.ResolveLinkedModules(parsed.Modules)
	if err != nil || len(linked.Errors) > 0 {
		t.Fatal(err, linked.Errors)
	}
	analyzed, err := quint.AnalyzeModules(parsed.Modules, linked.Table)
	if err != nil || len(analyzed.Diagnostics) > 0 {
		t.Fatal(err, analyzed.Diagnostics)
	}
	property := ""
	if len(temporal) > 0 {
		property = temporal[0]
	}
	compiled, err := quint.CompileModules(analyzed.Modules, quint.AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes}, parsed.IDs, parsed.Sources, quint.CompilationOptions{Main: "M", CompilationEntries: quint.CompilationEntries{Target: "json", Flatten: true, Invariants: []string{"safe"}, Temporal: property}})
	if err != nil || len(compiled.Diagnostics) > 0 || len(compiled.AnalysisDiagnostics) > 0 {
		t.Fatal(err, compiled.Diagnostics, compiled.AnalysisDiagnostics)
	}
	return compiled
}

func TestNativeTLCSource(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC solver corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		name, source string
		outcome      Outcome
	}{
		{"safe", "module M { var x:int action init=x'=0 action step=x'=1-x val safe=x>=0 }", Safe},
		{"violation", "module M { var x:int action init=x'=0 action step=x'=1-x val safe=x==0 }", Violation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := Check(ctx, compileSource(t, tc.source), Options{Workers: 1})
			if err != nil || result.Outcome != tc.outcome || !result.Complete {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if result.InitialStates != 1 || result.StatesGenerated < 1 || result.DistinctStates < 1 {
				t.Fatal(result)
			}
		})
	}
}

func withOneWorker(options Options) Options { options.Workers = 1; return options }
