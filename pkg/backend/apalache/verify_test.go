package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestInductiveVerificationProduction(t *testing.T) {
	t.Parallel()
	compiled := compileSource(t, "module M { var x:int action init=x'=0 action step=x'=1-x val ind=x.in(Set(0,1)) val safe=x>=0 }", "ind")
	result, err := New().Verify(context.Background(), compiled, nil, VerificationOptions{Inductive: true})
	if err != nil || result.Outcome != CheckSafe || len(result.Phases) != 3 {
		t.Fatal(result, err)
	}
}

func TestVerificationPhaseProtocol(t *testing.T) {
	compiled := quint.CompilationResult{MainModule: quint.Module{Declarations: []quint.Declaration{{Kind: "def", Name: "q::inductiveInv"}, {Kind: "def", Name: "q::inv"}}}}
	loaded := json.RawMessage(`{"checker":{"max-error":2}}`)
	for _, tc := range []struct {
		name       string
		inductive  bool
		invariants []string
		stop       int
		outcome    CheckOutcome
		want       []VerificationPhase
	}{
		{"bounded", false, nil, -1, CheckSafe, []VerificationPhase{BoundedPhase}},
		{"all", true, nil, -1, CheckSafe, []VerificationPhase{InitializationPhase, PreservationPhase, ImplicationPhase}},
		{"no-ordinary", true, []string{}, -1, CheckSafe, []VerificationPhase{InitializationPhase, PreservationPhase}},
		{"initial-failure", true, nil, 0, CheckViolation, []VerificationPhase{InitializationPhase}},
		{"preservation-failure", true, nil, 1, CheckViolation, []VerificationPhase{InitializationPhase, PreservationPhase}},
		{"implication-failure", true, nil, 2, CheckViolation, []VerificationPhase{InitializationPhase, PreservationPhase, ImplicationPhase}},
		{"deadlock", true, nil, 1, CheckDeadlock, []VerificationPhase{InitializationPhase, PreservationPhase}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := VerificationOptions{Inductive: tc.inductive, CheckOptions: CheckOptions{MaxSteps: 17, Next: "Next", Invariants: tc.invariants, Temporal: []string{"Live"}, RandomTransitions: true}}
			calls := 0
			check := func(ctx context.Context, spec quint.CompilationResult, config json.RawMessage, opts CheckOptions) (CheckResult, error) {
				if string(config) != string(loaded) || spec.MainModule.ID != compiled.MainModule.ID {
					t.Fatal("input changed")
				}
				if opts.Next != "Next" || !opts.RandomTransitions || !reflect.DeepEqual(opts.Temporal, []string{"Live"}) {
					t.Fatal("options lost", opts)
				}
				if tc.inductive {
					bounds := []int{0, 1, 0}
					if opts.MaxSteps != bounds[calls] {
						t.Fatal("incorrect phase bound", opts)
					}
					wantInit := ""
					if calls > 0 {
						wantInit = "q::inductiveInv"
					}
					if opts.Init != wantInit {
						t.Fatal("incorrect phase init", opts)
					}
					inv := "q::inductiveInv"
					if calls == 2 {
						inv = "q::inv"
					}
					if !reflect.DeepEqual(opts.Invariants, []string{inv}) {
						t.Fatal("incorrect obligation", opts)
					}
				} else if opts.MaxSteps != 17 {
					t.Fatal(opts)
				}
				outcome := CheckSafe
				if calls == tc.stop {
					outcome = tc.outcome
				}
				calls++
				return CheckResult{Outcome: outcome, Counterexamples: []json.RawMessage{json.RawMessage(`{"states":[]}`)}}, nil
			}
			result, err := verifyWithCheck(context.Background(), compiled, loaded, options, check)
			if err != nil || calls != len(tc.want) {
				t.Fatal(result, err, calls)
			}
			phases := []VerificationPhase{}
			for _, p := range result.Phases {
				phases = append(phases, p.Phase)
			}
			if !reflect.DeepEqual(phases, tc.want) || result.Outcome != tc.outcome {
				t.Fatal(result)
			}
			if options.MaxSteps != 17 || options.Init != "" || !reflect.DeepEqual(options.Invariants, tc.invariants) {
				t.Fatal("input options changed")
			}
		})
	}
}

func TestVerificationErrorsAndCancellation(t *testing.T) {
	compiled := quint.CompilationResult{MainModule: quint.Module{Declarations: []quint.Declaration{{Kind: "def", Name: "q::inductiveInv"}, {Kind: "def", Name: "q::inv"}}}}
	sentinel := errors.New("solver unknown")
	for _, at := range []int{0, 1, 2} {
		calls := 0
		result, err := verifyWithCheck(context.Background(), compiled, nil, VerificationOptions{Inductive: true}, func(context.Context, quint.CompilationResult, json.RawMessage, CheckOptions) (CheckResult, error) {
			call := calls
			calls++
			if call == at {
				return CheckResult{}, sentinel
			}
			return CheckResult{Outcome: CheckSafe}, nil
		})
		var phaseError *VerificationError
		if !errors.Is(err, sentinel) || !errors.As(err, &phaseError) || len(result.Phases) != at+1 || phaseError.Phase != result.Phases[at].Phase {
			t.Fatal(result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	result, err := verifyWithCheck(ctx, compiled, nil, VerificationOptions{Inductive: true}, func(context.Context, quint.CompilationResult, json.RawMessage, CheckOptions) (CheckResult, error) {
		calls++
		cancel()
		return CheckResult{Outcome: CheckSafe}, nil
	})
	if !errors.Is(err, context.Canceled) || calls != 1 || len(result.Phases) != 1 {
		t.Fatal(result, err)
	}
	if _, err = New().Verify(ctx, quint.CompilationResult{}, nil, VerificationOptions{}); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestVerificationMissingInvariantAndInvalidOutcome(t *testing.T) {
	called := false
	check := func(context.Context, quint.CompilationResult, json.RawMessage, CheckOptions) (CheckResult, error) {
		called = true
		return CheckResult{}, nil
	}
	if _, err := verifyWithCheck(context.Background(), quint.CompilationResult{}, nil, VerificationOptions{Inductive: true}, check); err == nil || called {
		t.Fatal("missing inductive wrapper checked", err)
	}
	if _, err := verifyWithCheck(context.Background(), quint.CompilationResult{}, nil, VerificationOptions{}, check); err == nil || !called {
		t.Fatal("invalid checker outcome accepted", err)
	}
	compiled := quint.CompilationResult{MainModule: quint.Module{Declarations: []quint.Declaration{{Kind: "def", Name: "q::inductiveInv"}}}}
	calls := 0
	options := VerificationOptions{Inductive: true, CheckOptions: CheckOptions{Invariants: []string{"A", "B"}, Init: "Start"}}
	result, err := verifyWithCheck(context.Background(), compiled, nil, options, func(_ context.Context, _ quint.CompilationResult, _ json.RawMessage, opts CheckOptions) (CheckResult, error) {
		if calls == 0 && opts.Init != "Start" {
			t.Fatal("explicit initial predicate lost")
		}
		if calls == 2 && !reflect.DeepEqual(opts.Invariants, []string{"A", "B"}) {
			t.Fatal("ordinary invariant selection lost", opts)
		}
		calls++
		return CheckResult{Outcome: CheckSafe}, nil
	})
	if err != nil || calls != 3 || result.Outcome != CheckSafe {
		t.Fatal(result, err)
	}
}
