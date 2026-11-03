package quint

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sort"
	"testing"
)

type statefulComparisonRegister struct {
	Name  string
	Value *Expr
}

type statefulComparisonCase struct {
	Name, Source, Expression, Category, Reason, Seed, SourceStatus string
	AnalysisErrors                                                 []Diagnostic
	Module                                                         Module
	Input                                                          Expr
	Typescript                                                     struct {
		Output        *Expr
		Error         *Diagnostic
		Thrown, Seed  string
		DebugOutput   string
		Record        Expr
		Current, Next []statefulComparisonRegister
		Trace         []Expr
		ReplShift     struct {
			Shifted            bool
			Missing            []string
			OldState, NewState Expr
		}
	}
	Rust            rustComparisonResponse
	RustSeed        string
	RustDebugOutput string
	RustShift       struct {
		Response    string
		Shifted     bool
		MissingVars []string        `json:"missing_vars"`
		OldState    json.RawMessage `json:"old_state"`
		NewState    json.RawMessage `json:"new_state"`
	}
	Differences []string
	RustInput   []json.RawMessage
}

// The default suite reads saved output from both actual reference runtimes. It
// neither starts Node/Rust nor builds an evaluator; every case has a fresh Go
// module execution and exercises the same single source expression.
func TestThreeRuntimeStatefulComparisonCorpus(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Metadata struct {
			RustPackage, RustBinarySHA256, CargoLockSHA256 string
			RustSourcesSHA256, TypescriptSourcesSHA256     map[string]string
			Protocol, Policy                               string
			Coverage                                       struct {
				Cases, ValidSource, RuntimeIROnly int
				Categories                        []string
			}
			Differences map[string]int
		}
		Cases []statefulComparisonCase
	}
	data, err := os.ReadFile("testdata/parser/runtime_stateful_comparison.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	metadata := fixture.Metadata
	for _, digest := range []string{metadata.RustBinarySHA256, metadata.CargoLockSHA256} {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != 32 {
			t.Fatal("missing pinned reference provenance", digest, err)
		}
	}
	if metadata.RustPackage != "quint_evaluator 0.2.0" || len(metadata.RustSourcesSHA256) != 4 || len(metadata.TypescriptSourcesSHA256) != 5 || metadata.Protocol == "" || metadata.Policy == "" {
		t.Fatal("missing reference source/protocol metadata")
	}
	if len(fixture.Cases) != 48 || metadata.Coverage.Cases != len(fixture.Cases) || metadata.Coverage.ValidSource != 38 || metadata.Coverage.RuntimeIROnly != 10 || len(metadata.Coverage.Categories) != 8 {
		t.Fatal("incomplete stateful coverage", metadata.Coverage)
	}
	counts := map[string]int{}
	validSource, runtimeIROnly := 0, 0
	for _, f := range fixture.Cases {
		if f.SourceStatus == "valid-source" && len(f.AnalysisErrors) == 0 {
			validSource++
		} else if f.SourceStatus == "runtime-ir-only" && len(f.AnalysisErrors) > 0 {
			runtimeIROnly++
		} else {
			t.Fatal("source validity classification inconsistent", f.Name, f.SourceStatus, f.AnalysisErrors)
		}
		for _, difference := range f.Differences {
			counts[difference]++
		}
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			if f.Name == "" || f.Source == "" || f.Expression == "" || f.Category == "" || f.Reason == "" || len(f.RustInput) != 4 || f.Rust.Response != "EvaluationResult" || f.RustShift.Response != "ReplShiftResult" {
				t.Fatal("missing source request, policy explanation or actual Rust responses")
			}
			if f.Typescript.Thrown != "" {
				t.Fatal("unclassified TypeScript exception", f.Typescript.Thrown)
			}
			before, err := json.Marshal(f.Module)
			if err != nil {
				t.Fatal(err)
			}
			seed, err := ParseInteger(f.Seed)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := NewModuleEvaluator(f.Module, NewRNG(seed), false)
			if err != nil {
				t.Fatal(err)
			}
			var debug bytes.Buffer
			ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: &debug, DebugWidth: 80})
			got, err := evaluator.EvaluateExpression(ctx, f.Input)
			if f.Typescript.Error != nil {
				want := f.Typescript.Error
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != want.Code || diagnostic.Message != want.Message || !reflect.DeepEqual(diagnostic.Reference, want.Reference) {
					t.Fatal("Go diagnostic", err, "TypeScript", want)
				}
			} else {
				if err != nil || f.Typescript.Output == nil {
					t.Fatal("Go evaluation", err)
				}
				assertStatefulComparisonValue(t, ctx, got, *f.Typescript.Output)
			}
			if debug.String() != f.Typescript.DebugOutput || evaluator.RNG.State().String() != f.Typescript.Seed {
				t.Fatalf("Go debug/counter %q/%s, TypeScript %q/%s", debug.String(), evaluator.RNG.State(), f.Typescript.DebugOutput, f.Typescript.Seed)
			}
			record, err := evaluator.Storage.Record()
			if err != nil {
				t.Fatal(err)
			}
			assertStatefulComparisonValue(t, ctx, record, f.Typescript.Record)
			assertStatefulComparisonRegisters(t, ctx, evaluator.Storage, f.Typescript.Current, false)
			assertStatefulComparisonRegisters(t, ctx, evaluator.Storage, f.Typescript.Next, true)
			states := evaluator.Trace.States()
			if len(states) != len(f.Typescript.Trace) {
				t.Fatal("Go trace states", len(states), "TypeScript", len(f.Typescript.Trace))
			}
			for i, state := range states {
				assertStatefulComparisonValue(t, ctx, state, f.Typescript.Trace[i])
			}
			checkStatefulBackendDifferences(t, ctx, f, got)
			assertStatefulComparisonValue(t, ctx, record, f.Typescript.ReplShift.OldState)
			changed := false
			missing := []string{}
			for _, register := range evaluator.Storage.next.All() {
				if register.Value.Valid() {
					changed = true
				} else {
					missing = append(missing, register.Name)
				}
			}
			if changed != f.Typescript.ReplShift.Shifted {
				t.Fatal("pending assignments disagree with TypeScript REPL shift")
			}
			if changed {
				if err := evaluator.Shift(); err != nil {
					t.Fatal(err)
				}
			} else {
				missing = []string{}
			}
			sort.Strings(missing)
			wantMissing := append([]string{}, f.Typescript.ReplShift.Missing...)
			sort.Strings(wantMissing)
			if !reflect.DeepEqual(missing, wantMissing) {
				t.Fatal("missing assignments", missing, "TypeScript", wantMissing)
			}
			record, err = evaluator.Storage.Record()
			if err != nil {
				t.Fatal(err)
			}
			assertStatefulComparisonValue(t, ctx, record, f.Typescript.ReplShift.NewState)
			after, err := json.Marshal(f.Module)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("module source IR mutated", err)
			}
		})
	}
	if validSource != metadata.Coverage.ValidSource || runtimeIROnly != metadata.Coverage.RuntimeIROnly {
		t.Fatal("source validity counts differ from saved metadata")
	}
	for kind, count := range metadata.Differences {
		if counts[kind] != count {
			t.Fatal("reference difference count", kind, counts[kind], count)
		}
	}
}

func assertStatefulComparisonValue(t *testing.T, ctx context.Context, got RuntimeValue, expression Expr) {
	t.Helper()
	want, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal("invalid saved value", err)
	}
	equal, err := got.EqualContext(ctx, want)
	if err != nil || !equal {
		t.Fatal("Go value", got.String(), "TypeScript", want.String(), err)
	}
}

func assertStatefulComparisonRegisters(t *testing.T, ctx context.Context, storage *VariableStorage, expected []statefulComparisonRegister, next bool) {
	t.Helper()
	registers := storage.current
	if next {
		registers = storage.next
	}
	byName := map[string]RuntimeValue{}
	for _, register := range registers.All() {
		byName[register.Name] = register.Value
	}
	if len(byName) != len(expected) {
		t.Fatal("Go register discovery", len(byName), "TypeScript", len(expected), "next", next)
	}
	for _, register := range expected {
		got, exists := byName[register.Name]
		if !exists || got.Valid() != (register.Value != nil) {
			t.Fatal("Go register", register.Name, got.String(), "TypeScript", register.Value, "next", next)
		}
		if register.Value != nil {
			assertStatefulComparisonValue(t, ctx, got, *register.Value)
		}
	}
}

func checkStatefulBackendDifferences(t *testing.T, ctx context.Context, f statefulComparisonCase, got RuntimeValue) {
	t.Helper()
	comparison := rustComparisonCase{Input: f.Input, Rust: f.Rust, RustSeed: f.RustSeed, RustDebugOutput: f.RustDebugOutput}
	comparison.Typescript.Output = f.Typescript.Output
	comparison.Typescript.Error = f.Typescript.Error
	comparison.Typescript.Seed = f.Typescript.Seed
	comparison.Typescript.DebugOutput = f.Typescript.DebugOutput
	pendingDiffers := false
	for _, difference := range f.Differences {
		if difference == "pending-state" {
			if pendingDiffers {
				t.Fatal("duplicate pending-state difference")
			}
			pendingDiffers = true
		} else {
			comparison.Differences = append(comparison.Differences, difference)
		}
	}
	checkRustComparisonDifferences(t, ctx, comparison, got)
	actualDifference := f.Typescript.ReplShift.Shifted != f.RustShift.Shifted
	if f.Typescript.ReplShift.Shifted && f.RustShift.Shifted {
		for _, pair := range []struct {
			typescript Expr
			rust       json.RawMessage
		}{{f.Typescript.ReplShift.OldState, f.RustShift.OldState}, {f.Typescript.ReplShift.NewState, f.RustShift.NewState}} {
			want, err := NewRuntimeValue(pair.typescript)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := want.EqualContext(ctx, decodeRustComparisonValue(t, pair.rust))
			if err != nil {
				t.Fatal(err)
			}
			actualDifference = actualDifference || !equal
		}
	}
	if actualDifference != pendingDiffers {
		t.Fatal("saved pending-state backend classification is inconsistent")
	}
}
