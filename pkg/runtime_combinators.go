package quint

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

// RuntimeSimulationCall is the lazy top-level q::test or q::testOnce request.
// The action and invariant expressions are passed to the simulation driver
// unevaluated. Nested applications are ordinary unsupported builtin calls.
type RuntimeSimulationCall struct {
	Options               SimulationOptions
	Init, Step, Invariant Expr
}

// RuntimeSimulationHook executes one request using caller-owned state and RNG.
type RuntimeSimulationHook func(context.Context, RuntimeSimulationCall) (SimulationOutcome, error)

// EvaluateExpression evaluates IR under this module's persistent definitions.
// It recognizes q::test/q::testOnce only at the request's outermost application;
// ordinary recursive evaluation retains their unsupported-builtin behavior.
func (e *ModuleEvaluator) EvaluateExpression(ctx context.Context, expression Expr) (RuntimeValue, error) {
	if expression.Kind == "app" && RuntimeCombinatorArity(expression.Opcode) != 0 {
		return EvalRuntimeCombinator(ctx, expression, e.SimulateExpressions)
	}
	return e.evaluateExpression(ctx, expression)
}

// SimulateExpressions runs unevaluated init/step/invariant expressions with the
// same state, RNG, definition caches, trace selection, and cancellation policy as
// Simulate. Witnesses, when requested, are names in the existing module.
func (e *ModuleEvaluator) SimulateExpressions(ctx context.Context, call RuntimeSimulationCall) (SimulationOutcome, error) {
	options := call.Options
	if err := ctx.Err(); err != nil {
		return SimulationOutcome{}, err
	}
	if options.MaxSamples < 0 || options.MaxSteps < 0 || options.NumberOfTraces < 0 {
		return SimulationOutcome{}, fmt.Errorf("simulation limits must be nonnegative")
	}
	for _, name := range options.Witnesses {
		if _, ok := e.definitions.Get2(name); !ok {
			return SimulationOutcome{}, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name}
		}
	}
	// Compilation discovers picks even if zero limits prevent runtime evaluation.
	for _, expression := range []Expr{call.Init, call.Step, call.Invariant} {
		if err := e.prepareExpression(ctx, expression); err != nil {
			return SimulationOutcome{}, err
		}
	}
	for _, name := range options.Witnesses {
		if err := e.PrepareDefinition(ctx, name); err != nil {
			return SimulationOutcome{}, err
		}
	}
	// These names cannot collide with names written in Quint source.
	options.Init, options.Step, options.Invariant = "\x00init", "\x00step", "\x00invariant"
	return e.simulate(ctx, options, func(name string) (RuntimeValue, error) {
		switch name {
		case options.Init:
			return e.evaluateExpression(ctx, call.Init)
		case options.Step:
			return e.evaluateExpression(ctx, call.Step)
		case options.Invariant:
			return e.evaluateExpression(ctx, call.Invariant)
		default:
			return e.Evaluate(ctx, name, nil)
		}
	})
}

// RuntimeCombinatorArity identifies the two top-level simulation entry points.
func RuntimeCombinatorArity(op string) int {
	switch op {
	case "q::test":
		return 6
	case "q::testOnce":
		return 5
	}
	return 0
}

// EvalRuntimeCombinator handles the evaluator-level simulation shortcut.
// Limits are integer literals in IR, rather than evaluated expressions, matching
// the reference. Negative limits perform no corresponding iterations. Go int
// overflow is reported explicitly instead of wrapping a resource limit.
func EvalRuntimeCombinator(ctx context.Context, expression Expr, simulate RuntimeSimulationHook) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	fail := func(code, message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: code, Message: message, Reference: &expression.ID}
	}
	arity := RuntimeCombinatorArity(expression.Opcode)
	if expression.Kind != "app" || arity == 0 {
		return fail("QNT000", "Unknown builtin "+expression.Opcode)
	}
	if len(expression.Args) != arity {
		return fail("QNT501", fmt.Sprintf("%s requires %d arguments", expression.Opcode, arity))
	}
	limit := func(value Expr) (int, error) {
		if value.Kind != "int" {
			return 0, Diagnostic{Code: "QNT501", Message: "Expected an integer", Reference: &value.ID}
		}
		n, _ := value.Int.Big().Float64()
		if n <= 0 {
			return 0, nil
		}
		// int's positive endpoint is not representable as float64 on 64-bit
		// hosts. Compare against its exact exclusive power-of-two boundary.
		if math.IsInf(n, 1) || n >= math.Ldexp(1, strconv.IntSize-1) {
			return 0, Diagnostic{Code: "QNT501", Message: "Simulation limit exceeds Go int capacity", Reference: &value.ID}
		}
		return int(n), nil
	}
	args := expression.Args
	call := RuntimeSimulationCall{Options: SimulationOptions{MaxSamples: 1, Witnesses: []string{}}}
	if expression.Opcode == "q::test" {
		var err error
		call.Options.MaxSamples, err = limit(args[0])
		if err != nil {
			return RuntimeValue{}, err
		}
		args = args[1:]
	}
	var err error
	call.Options.MaxSteps, err = limit(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	call.Options.NumberOfTraces, err = limit(args[1])
	if err != nil {
		return RuntimeValue{}, err
	}
	call.Init, call.Step, call.Invariant = copyRuntimeExpression(args[2]), copyRuntimeExpression(args[3]), copyRuntimeExpression(args[4])
	if simulate == nil {
		return fail("QNT501", "Simulation requires an evaluator")
	}
	outcome, err := simulate(ctx, call)
	if err != nil {
		return RuntimeValue{}, err
	}
	if outcome.Status == "error" {
		if len(outcome.Errors) == 0 {
			return fail("QNT501", "Simulation failed without a diagnostic")
		}
		return RuntimeValue{}, outcome.Errors[0]
	}
	return NewRuntimeValue(Expr{Kind: "str", Str: outcome.Status})
}
