package apalache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pal "github.com/glycerine/apalache-go/pkg"
	cmd "github.com/glycerine/apalache-go/pkg/shai/cmdexecutor"
)

// The command service's CHECK response does not distinguish every bounded
// outcome. Retain its strict request decoder and production frontend, then run
// the exported checker once and encode every outcome explicitly here.
//
// This mirrors the service's checker configuration, including encoding/json's
// case folding and repeated-object merge semantics. The service still validates
// the complete schema, source policy, features, and typechecker options.
type commandCheckerOptions struct {
	Tuning             map[string]string `json:"tuning"`
	Algorithm          *string           `json:"algo"`
	TLCConfig          *string           `json:"config"`
	DiscardDisabled    *bool             `json:"discard-disabled"`
	ConstantInit       *string           `json:"cinit"`
	Init               *string           `json:"init"`
	Invariants         []string          `json:"inv"`
	Next               *string           `json:"next"`
	Length             *int              `json:"length"`
	MaxError           *int              `json:"max-error"`
	TimeoutSMT         *int              `json:"timeout-smt"`
	NoDeadlock         *bool             `json:"no-deadlock"`
	SMTSolver          *string           `json:"smt-solver"`
	SMTEncoding        *string           `json:"smt-encoding"`
	TemporalProperties []string          `json:"temporal"`
	View               *string           `json:"view"`
}

func (e *Executor) runCheck(ctx context.Context, config json.RawMessage) (*cmd.CmdResponse, error) {
	response, err := e.service.Run(ctx, &cmd.CmdRequest{Cmd: cmd.Cmd_TYPECHECK, Config: string(config)})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var effective struct {
		Checker commandCheckerOptions `json:"checker"`
	}
	// A successful service decode is authoritative. This narrower view cannot
	// introduce an additional accepted field or reinterpret checker scalars.
	if err := json.Unmarshal(config, &effective); err != nil {
		return unexpectedCheckResponse(err), nil
	}
	checker := effective.Checker
	if err := checker.validate(); err != nil {
		// Native CHECK validates these options before parsing/typechecking.
		// Preserve that precedence over a frontend PASS_FAILURE response.
		if response != nil && response.GetFailure() != nil && response.GetFailure().GetErrorType() == cmd.CmdErrorType_UNEXPECTED {
			return response, nil // source materialization precedes validation
		}
		return unexpectedCheckResponse(err), nil
	}
	if response == nil || response.GetSuccess() == "" {
		return response, nil
	}
	sources := pal.NewSourceStore()
	module, err := pal.UnmarshalSingleModuleIRJSON([]byte(response.GetSuccess()), sources)
	if err != nil {
		return unexpectedCheckResponse(err), nil
	}
	// Canonical IR JSON carries the inferred types, but not pass properties.
	// This module came from the real TYPECHECK command immediately above.
	module.AddProperty(pal.PropertyTypeChecked)
	prepared, err := pal.PrepareSafetyTransitions(module, pal.SafetyPipelineOptions{
		Specification: pal.SpecificationConfiguration{
			Behavior:     pal.InitNextSpec{Init: checkString(checker.Init, "Init"), Next: checkString(checker.Next, "Next")},
			ConstantInit: checkString(checker.ConstantInit, ""), Invariants: checker.Invariants,
			TemporalProperties: checker.TemporalProperties, View: checkString(checker.View, ""),
		},
		Sources: sources,
	})
	if err != nil {
		return unexpectedCheckResponse(err), nil
	}
	tuning, err := pal.ParseCheckerTuning(checker.Tuning)
	if err != nil {
		return unexpectedCheckResponse(checkerTuningError(err)), nil
	}
	encoding := pal.SMTEncodingOOPSLA19
	if checker.SMTEncoding != nil {
		encoding, err = pal.ParseSMTEncoding(*checker.SMTEncoding)
		if err != nil {
			return unexpectedCheckResponse(err), nil
		}
	}
	if checker.TimeoutSMT != nil {
		ctx = pal.WithSMTQueryTimeout(ctx, time.Duration(*checker.TimeoutSMT)*time.Second)
	}
	discardDisabled := checker.DiscardDisabled == nil || *checker.DiscardDisabled
	options := pal.BoundedSafetyOptions{
		MaxSteps: checkInt(checker.Length, 10), MaxErrors: checkInt(checker.MaxError, 1),
		CheckDeadlocks:  checker.NoDeadlock == nil || !*checker.NoDeadlock,
		DiscardDisabled: &discardDisabled, SMTEncoding: encoding, Tuning: tuning,
	}
	if tuning.Simulation() {
		// search.simulation samples random runs seeded by search.simulation.seed.
		// A clean simulation is not an exhaustive bounded proof.
		report, err := pal.SimulateAndCheckWithOptions(ctx, prepared.Module, prepared.Predicates, pal.SimulationOptions{
			MaxSteps: options.MaxSteps, MaxRuns: tuning.SimulationRuns(), Randomize: true,
			RandomSeed: tuning.SimulationSeed(), SMTEncoding: encoding, Tuning: tuning, MaxErrors: options.MaxErrors,
		})
		if err != nil {
			return unexpectedCheckResponse(err), nil
		}
		result := pal.BoundedSafetyResult{Outcome: pal.SafetyNoCounterexample}
		if len(report.Counterexamples) != 0 {
			result.Outcome = pal.SafetyCounterexample
			result.Counterexamples = report.Counterexamples
		}
		return encodeBoundedCheckResponse(prepared.Module, result)
	}
	result, err := pal.CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates, options)
	if err != nil {
		return unexpectedCheckResponse(err), nil
	}
	return encodeBoundedCheckResponse(prepared.Module, result)
}

func (checker commandCheckerOptions) validate() error {
	if _, err := pal.ParseCheckerTuning(checker.Tuning); err != nil {
		return fmt.Errorf("invalid checker tuning: %w", checkerTuningError(err))
	}
	if checker.Algorithm != nil && !strings.EqualFold(*checker.Algorithm, "incremental") {
		return fmt.Errorf("unsupported checker algorithm %q (Go port currently supports incremental)", *checker.Algorithm)
	}
	if checker.SMTEncoding != nil {
		if _, err := pal.ParseSMTEncoding(*checker.SMTEncoding); err != nil {
			return err
		}
	}
	if checker.SMTSolver != nil && !strings.EqualFold(*checker.SMTSolver, "z3") {
		return fmt.Errorf("unsupported SMT solver %q (Go port uses g3 as its Z3 backend)", *checker.SMTSolver)
	}
	if checker.MaxError != nil && *checker.MaxError <= 0 {
		return fmt.Errorf("checker.max-error must be positive")
	}
	if checker.MaxError != nil && *checker.MaxError > 1 && checker.View == nil {
		return fmt.Errorf("checker.max-error=%d requires checker.view", *checker.MaxError)
	}
	if checker.Length != nil && *checker.Length < 0 {
		return fmt.Errorf("checker.length cannot be negative")
	}
	if checker.TimeoutSMT != nil && *checker.TimeoutSMT < 0 {
		return fmt.Errorf("checker.timeout-smt cannot be negative")
	}
	if checker.TimeoutSMT != nil && int64(*checker.TimeoutSMT) > int64(time.Duration(1<<63-1)/time.Second) {
		return fmt.Errorf("checker.timeout-smt is too large")
	}
	if (checker.Init == nil) != (checker.Next == nil) {
		return fmt.Errorf("checker.init and checker.next must be provided together")
	}
	return nil
}

func encodeBoundedCheckResponse(module *pal.Module, result pal.BoundedSafetyResult) (*cmd.CmdResponse, error) {
	if result.Outcome == pal.SafetyNoCounterexample {
		encoded, err := pal.MarshalSingleModuleIRJSON(module, pal.IRJSONOptions{})
		if err != nil {
			return unexpectedCheckResponse(err), nil
		}
		return &cmd.CmdResponse{Result: &cmd.CmdResponse_Success{Success: string(encoded)}}, nil
	}
	var status string
	counterexamples := []json.RawMessage{}
	switch result.Outcome {
	case pal.SafetyCounterexample:
		status = "Error"
		for _, counterexample := range result.Counterexamples {
			encoded, err := pal.MarshalDecodedTraceITF(module, counterexample.States, pal.ITFOptions{})
			if err != nil {
				return unexpectedCheckResponse(fmt.Errorf("encode checker counterexample: %w", err)), nil
			}
			counterexamples = append(counterexamples, encoded)
		}
	case pal.SafetyDeadlock:
		status = "Deadlock"
		if len(result.States) != 0 {
			encoded, err := pal.MarshalDecodedTraceITF(module, result.States, pal.ITFOptions{})
			if err != nil {
				return unexpectedCheckResponse(fmt.Errorf("encode checker deadlock: %w", err)), nil
			}
			counterexamples = append(counterexamples, encoded)
		}
	case pal.SafetyUnknown:
		status = "Unknown"
	case pal.SafetyExecutionsTooShort:
		status = "ExecutionsTooShort"
	default:
		return unexpectedCheckResponse(fmt.Errorf("unrecognized Go checker outcome %d", result.Outcome)), nil
	}
	errorData := map[string]any{"checking_result": status, "counterexamples": counterexamples}
	if result.Outcome == pal.SafetyCounterexample {
		errorData["nerrors"] = len(counterexamples)
	}
	encoded, err := json.Marshal(struct {
		Pass  string         `json:"pass_name"`
		Error map[string]any `json:"error_data"`
		Exit  int            `json:"exit_code"`
	}{"BoundedChecker", errorData, 12})
	if err != nil {
		return unexpectedCheckResponse(err), nil
	}
	return &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{ErrorType: cmd.CmdErrorType_PASS_FAILURE, Data: string(encoded)}}}, nil
}

func unexpectedCheckResponse(err error) *cmd.CmdResponse {
	encoded, _ := json.Marshal(struct {
		Message string   `json:"msg"`
		Stack   []string `json:"stack_trace"`
	}{err.Error(), []string{}})
	return &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{ErrorType: cmd.CmdErrorType_UNEXPECTED, Data: string(encoded)}}}
}

func checkString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func checkInt(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}
