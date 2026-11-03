package tlc

import (
	"context"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	native "github.com/glycerine/tlago/tlc"
)

func TestNativeTLCEvaluationErrorsAreFailures(t *testing.T) {
	for _, test := range []struct {
		name, definitions, invariant, property string
		code                                   int
	}{
		{"assumption evaluation", "ASSUME 1 \\div 0 = 0", "", "", native.ECTLCAssumptionEvaluationError},
		{"initial invariant evaluation", "Inv == 1 \\div 0 = IF x THEN 1 ELSE 0", "Inv", "", native.ECTLCNestedExpression},
		{"next invariant evaluation", "Inv == ~x \\/ (1 \\div 0 = 0)", "Inv", "", native.ECTLCInvariantEvaluationFailed},
		{"action property evaluation", "Live == [][x' /\\ (1 \\div 0 = 0)]_x", "", "Live", native.ECTLCModuleDivisionByZero},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := Options{Workers: 1}
			if test.invariant != "" {
				options.Invariants = []string{test.invariant}
			}
			if test.property != "" {
				options.Temporal = []string{test.property}
			}
			source := "---- MODULE M ----\nEXTENDS Integers\nVARIABLE x\nq_init == x=FALSE\nq_step == x'=~x\n" + test.definitions + "\n===="
			result, err := CheckTLA(context.Background(), "M.tla", source, options)
			if err == nil || result.Complete || result.Outcome != "" {
				t.Fatalf("evaluation reported a definitive verdict: %+v, %v", result, err)
			}
			if test.code != 0 && result.ErrorCode != test.code {
				t.Fatalf("error code %d; want %d; result=%+v err=%v", result.ErrorCode, test.code, result, err)
			}
			if !strings.Contains(result.Log, "second argument of \\div is 0") {
				t.Fatal("lost native evaluation diagnostic", result.Log, err)
			}
		})
	}
}

func TestNativeTLCFalsePredicatesRemainViolations(t *testing.T) {
	for _, test := range []struct {
		name, definitions, invariant, property string
		code                                   int
		traceLength                            int
	}{
		{"false assumption", "ASSUME FALSE", "", "", native.ECTLCAssumptionFalse, 0},
		{"false initial invariant", "Inv == x", "Inv", "", native.ECTLCInvariantViolatedInitial, 1},
		{"false next invariant", "Inv == ~x", "Inv", "", native.ECTLCInvariantViolatedBehavior, 2},
		{"false initial property", "Live == x", "", "Live", native.ECTLCPropertyViolatedInitial, 1},
		{"false action property", "Live == [][x'=x]_x", "", "Live", native.ECTLCActionPropertyViolatedBehavior, 2},
		{"false assertion", "q_step == Assert(FALSE,\"failure\") /\\ x'=x", "", "", native.ECTLCValueAssertFailed, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := Options{Workers: 1}
			if test.invariant != "" {
				options.Invariants = []string{test.invariant}
			}
			if test.property != "" {
				options.Temporal = []string{test.property}
			}
			next, extends := "q_step == x'=~x\n", ""
			if test.name == "false assertion" {
				next = ""
				extends = "EXTENDS TLC\n"
			}
			source := "---- MODULE M ----\n" + extends + "VARIABLE x\nq_init == x=FALSE\n" + next + test.definitions + "\n===="
			result, err := CheckTLA(context.Background(), "M.tla", source, options)
			if err != nil || !result.Complete || result.Outcome != Violation || result.ErrorCode != test.code {
				t.Fatalf("false predicate lost its violation: %+v, %v", result, err)
			}
			if test.traceLength != 0 && len(result.Trace) != test.traceLength {
				t.Fatalf("got %d trace states; want %d: %+v", len(result.Trace), test.traceLength, result)
			}
			if result.Loop != nil {
				t.Fatal("finite-prefix violation invented a lasso", result)
			}
		})
	}
}

func TestNativeTLCInitialPropertyTypedTrace(t *testing.T) {
	source := strings.Replace(booleanTLA, "Bad == ~x", "Bad == x", 1)
	result, err := checkTLA(context.Background(), "M.tla", source, Options{Workers: 1, Temporal: []string{"Bad"}, ITF: true}, []itfVariable{{name: "original::x", tlaName: "x", typ: quint.Type{Kind: "bool"}}})
	if err != nil || result.Outcome != Violation || !result.Complete || result.ITFError != "" {
		t.Fatal(result, err)
	}
	trace, err := quint.DecodeITFTrace(result.ITF)
	if err != nil || len(trace.States) != 1 || trace.Loop != nil || len(trace.Vars) != 1 || trace.Vars[0] != "original::x" || trace.States[0].Args[1].Bool {
		t.Fatal(string(result.ITF), err)
	}
}

func TestNativeTLCEvaluationExitStatusDistinctions(t *testing.T) {
	for _, code := range []int{native.ECTLCAssumptionEvaluationError, native.ECTLCPostconditionEvaluationError, native.ECTLCActionPropertyEvaluationFailed} {
		if !evaluationFailure(code) {
			t.Fatal("evaluation error treated as violation", code)
		}
	}
	for _, code := range []int{
		native.NoError, native.ECTLCAssumptionFalse, native.ECTLCPostconditionFalse,
		native.ECTLCPossibleUnwitnessed, native.ECTLCDeadlockReached,
		native.ECTLCInvariantViolatedInitial, native.ECTLCInvariantViolatedBehavior,
		native.ECTLCActionPropertyViolatedBehavior, native.ECTLCPropertyViolatedInitial,
		native.ECTLCTemporalPropertyViolated, native.ECTLCValueAssertFailed,
	} {
		if evaluationFailure(code) {
			t.Fatal("genuine result treated as evaluation failure", code)
		}
	}
}
