package apalache

import (
	"context"
	"encoding/json"
	"fmt"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	quint "github.com/glycerine/quint-go/pkg"
)

// CheckOptions overrides loaded checker configuration as Quint's CLI does.
// MaxSteps is an explicit bound. Empty Init/Next use the compiled q::* wrappers.
// Nil Invariants selects q::inv if present; an empty slice checks only deadlocks.
// Temporal and RandomTransitions replace their loaded configuration values.
type CheckOptions struct {
	MaxSteps             int
	Init, Next           string
	Invariants, Temporal []string
	RandomTransitions    bool
}

type CheckOutcome string

const (
	CheckSafe      CheckOutcome = "safe"
	CheckViolation CheckOutcome = "violation"
	CheckDeadlock  CheckOutcome = "deadlock"
)

// CheckResult describes bounded checking, not an unbounded proof. Counterexample
// ITF JSON remains lossless, including arbitrary integers and loop metadata.
type CheckResult struct {
	Outcome         CheckOutcome
	Counterexamples []json.RawMessage
	Data            json.RawMessage
}

// CheckConfig merges loaded JSON with compiled input and explicit options.
// It retains other settings for the Go service to validate, including unsupported
// options; they are never silently removed. Unknown tuning keys also reach it.
func CheckConfig(compiled quint.CompilationResult, loaded json.RawMessage, options CheckOptions) (json.RawMessage, error) {
	if options.MaxSteps < 0 {
		return nil, fmt.Errorf("maximum steps cannot be negative")
	}
	if err := validateTemporalNames(compiled, options); err != nil {
		return nil, err
	}
	input, err := MarshalInput(compiled)
	if err != nil {
		return nil, err
	}
	config, err := readConfigObject(loaded)
	if err != nil {
		return nil, fmt.Errorf("Apalache configuration: %w", err)
	}
	checker, err := readConfigObject(config.values.Get("checker"))
	if err != nil {
		return nil, fmt.Errorf("Apalache checker configuration: %w", err)
	}
	tuning, err := readConfigObject(checker.values.Get("tuning"))
	if err != nil {
		return nil, fmt.Errorf("Apalache tuning configuration: %w", err)
	}
	init, next := options.Init, options.Next
	if init == "" {
		init = "q::init"
	}
	if next == "" {
		next = "q::step"
	}
	invariants := options.Invariants
	if invariants == nil {
		invariants = []string{}
		for _, d := range compiled.MainModule.Declarations {
			if d.Kind == "def" && d.Name == "q::inv" {
				invariants = append(invariants, d.Name)
				break
			}
		}
	}
	if err = config.set("source", quintSource{Kind: "string", Format: "qnt", Content: string(input)}); err != nil {
		return nil, err
	}
	for _, entry := range []struct {
		name  string
		value any
	}{{"length", options.MaxSteps}, {"init", init}, {"next", next}, {"inv", invariants}} {
		if err = checker.set(entry.name, entry.value); err != nil {
			return nil, err
		}
	}
	// Upstream assigns undefined for absent temporal flags, removing loaded values.
	if len(options.Temporal) == 0 {
		checker.values.Del("temporal")
	} else if err = checker.set("temporal", options.Temporal); err != nil {
		return nil, err
	}
	simulation := "false"
	if options.RandomTransitions {
		simulation = "true"
	}
	if err = tuning.set("search.simulation", simulation); err != nil {
		return nil, err
	}
	if err = checker.set("tuning", tuning); err != nil {
		return nil, err
	}
	if err = config.set("checker", checker); err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

// Check uses the production Go command service. Violations and deadlocks are
// returned as results; configuration, unsupported queries, and service failures
// return errors, preserving the backend payload in CommandFailure.
func (e *Executor) Check(ctx context.Context, compiled quint.CompilationResult, loaded json.RawMessage, options CheckOptions) (CheckResult, error) {
	if err := ctx.Err(); err != nil {
		return CheckResult{}, err
	}
	config, err := CheckConfig(compiled, loaded, options)
	if err != nil {
		return CheckResult{}, err
	}
	response, err := e.Run(ctx, cmd.Cmd_CHECK, config)
	if err != nil {
		return CheckResult{}, err
	}
	return decodeCheckResponse(response)
}

func decodeCheckResponse(response *cmd.CmdResponse) (CheckResult, error) {
	if response == nil {
		return CheckResult{}, fmt.Errorf("Go Apalache returned no check response")
	}
	switch result := response.Result.(type) {
	case *cmd.CmdResponse_Success:
		var payload struct {
			Modules []json.RawMessage `json:"modules"`
		}
		if err := json.Unmarshal([]byte(result.Success), &payload); err != nil || len(payload.Modules) != 1 {
			return CheckResult{}, fmt.Errorf("invalid Go Apalache check response")
		}
		return CheckResult{Outcome: CheckSafe, Data: json.RawMessage(result.Success)}, nil
	case *cmd.CmdResponse_Failure:
		if result.Failure == nil {
			return CheckResult{}, fmt.Errorf("Go Apalache returned an empty check failure")
		}
		failure := &CommandFailure{Type: result.Failure.GetErrorType().String(), Data: result.Failure.GetData()}
		if result.Failure.GetErrorType() != cmd.CmdErrorType_PASS_FAILURE {
			return CheckResult{}, failure
		}
		var payload struct {
			Pass  string          `json:"pass_name"`
			Error json.RawMessage `json:"error_data"`
		}
		if err := json.Unmarshal([]byte(failure.Data), &payload); err != nil {
			return CheckResult{}, fmt.Errorf("invalid Go Apalache failure response (%v): %w", err, failure)
		}
		if payload.Pass != "BoundedChecker" {
			return CheckResult{}, failure
		}
		var status struct {
			Result          string            `json:"checking_result"`
			Counterexamples []json.RawMessage `json:"counterexamples"`
		}
		if err := json.Unmarshal(payload.Error, &status); err != nil {
			return CheckResult{}, fmt.Errorf("invalid Go Apalache bounded-check failure (%v): %w", err, failure)
		}
		outcome := CheckViolation
		switch status.Result {
		case "Error":
		case "Deadlock":
			outcome = CheckDeadlock
		default:
			return CheckResult{}, failure
		}
		return CheckResult{Outcome: outcome, Counterexamples: status.Counterexamples, Data: json.RawMessage(failure.Data)}, nil
	default:
		return CheckResult{}, fmt.Errorf("Go Apalache check response has no result")
	}
}
