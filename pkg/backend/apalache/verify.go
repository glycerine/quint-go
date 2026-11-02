package apalache

import (
	"context"
	"encoding/json"
	"fmt"

	quint "github.com/glycerine/quint-go/pkg"
)

type VerificationPhase string

const (
	BoundedPhase        VerificationPhase = "bounded"
	InitializationPhase VerificationPhase = "initialization"
	PreservationPhase   VerificationPhase = "preservation"
	ImplicationPhase    VerificationPhase = "implication"
)

// VerificationOptions enables the inductive proof sequence in place of ordinary
// bounded checking. Compile with an InductiveInvariant expression first.
type VerificationOptions struct {
	CheckOptions
	Inductive bool
}

type VerificationPhaseResult struct {
	Phase  VerificationPhase
	Result CheckResult
}

// VerificationResult includes each attempted phase and the final checker result.
// Any non-safe outcome or error stops the sequence immediately.
type VerificationResult struct {
	CheckResult
	Phases []VerificationPhaseResult
}

// VerificationError associates a checker/service error with its proof obligation.
type VerificationError struct {
	Phase VerificationPhase
	Err   error
}

func (e *VerificationError) Error() string { return fmt.Sprintf("%s verification: %v", e.Phase, e.Err) }
func (e *VerificationError) Unwrap() error { return e.Err }

// Verify performs bounded checking or the upstream inductive-invariant protocol:
// initial inclusion at bound zero, preservation at bound one from the inductive
// predicate, then (if requested) ordinary-invariant implication at bound zero.
func (e *Executor) Verify(ctx context.Context, compiled quint.CompilationResult, loaded json.RawMessage, options VerificationOptions) (VerificationResult, error) {
	return verifyWithCheck(ctx, compiled, loaded, options, e.Check)
}

type checkFunction func(context.Context, quint.CompilationResult, json.RawMessage, CheckOptions) (CheckResult, error)

func verifyWithCheck(ctx context.Context, compiled quint.CompilationResult, loaded json.RawMessage, options VerificationOptions, check checkFunction) (VerificationResult, error) {
	result := VerificationResult{Phases: []VerificationPhaseResult{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	run := func(phase VerificationPhase, opts CheckOptions) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, &VerificationError{Phase: phase, Err: err}
		}
		checked, err := check(ctx, compiled, loaded, opts)
		result.Phases = append(result.Phases, VerificationPhaseResult{Phase: phase, Result: checked})
		result.CheckResult = checked
		if err != nil {
			return false, &VerificationError{Phase: phase, Err: err}
		}
		if checked.Outcome != CheckSafe && checked.Outcome != CheckViolation && checked.Outcome != CheckDeadlock {
			return false, &VerificationError{Phase: phase, Err: fmt.Errorf("invalid checker outcome %q", checked.Outcome)}
		}
		return checked.Outcome == CheckSafe, nil
	}
	if !options.Inductive {
		_, err := run(BoundedPhase, options.CheckOptions)
		return result, err
	}
	hasInductive := false
	ordinary := options.Invariants
	if ordinary == nil {
		ordinary = []string{}
	}
	for _, d := range compiled.MainModule.Declarations {
		if d.Kind != "def" {
			continue
		}
		if d.Name == "q::inductiveInv" {
			hasInductive = true
		}
		if options.Invariants == nil && d.Name == "q::inv" {
			ordinary = append(ordinary, d.Name)
		}
	}
	if !hasInductive {
		return result, fmt.Errorf("inductive verification requires compiled q::inductiveInv")
	}
	init := options.CheckOptions
	init.MaxSteps = 0
	init.Invariants = []string{"q::inductiveInv"}
	proceed, err := run(InitializationPhase, init)
	if err != nil || !proceed {
		return result, err
	}
	preservation := init
	preservation.MaxSteps = 1
	preservation.Init = "q::inductiveInv"
	proceed, err = run(PreservationPhase, preservation)
	if err != nil || !proceed || len(ordinary) == 0 {
		return result, err
	}
	implication := preservation
	implication.MaxSteps = 0
	implication.Invariants = append([]string{}, ordinary...)
	_, err = run(ImplicationPhase, implication)
	return result, err
}
