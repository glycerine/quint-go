package quint

import (
	"context"
	"math/big"
	"os"
)

// RuntimeNondetOptions controls retrying a false nondeterministic body. Every
// position bound must be strictly smaller than RetrySmallerThan to allow a
// retry. Infinite bounds disable retries. A zero or negative limit disables
// retries when at least one finite bound is present.
type RuntimeNondetOptions struct {
	RetrySmallerThan Integer
}

// EvalNondet evaluates a nondeterministic binding using the reference's default
// retry limit of 100, overridden by QUINT_RETRY_NONDET_SMALLER_THAN when set.
// The set expression runs once. bodyEval receives the cached pick (including
// any error), so callers can bind its name lazily and clear that binding after
// each invocation. Finite retries advance positions cyclically without drawing
// more random numbers or restoring pending state between attempts.
func EvalNondet(ctx context.Context, name string, storage *VariableStorage, rng *RNG,
	setEval func(context.Context) (RuntimeValue, error),
	bodyEval func(context.Context, RuntimeValue, error) (RuntimeValue, error),
) (RuntimeValue, error) {
	limit, _ := ParseInteger("100")
	if text, exists := os.LookupEnv("QUINT_RETRY_NONDET_SMALLER_THAN"); exists {
		var err error
		limit, err = ParseSeed(text)
		if err != nil {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "QUINT_RETRY_NONDET_SMALLER_THAN must be a big integer, found: " + text}
		}
	}
	return EvalNondetWithOptions(ctx, name, storage, rng, setEval, bodyEval, RuntimeNondetOptions{RetrySmallerThan: limit})
}

// EvalNondetWithOptions uses an explicit retry limit for an execution. The
// caller owns storage, RNG, and callback scopes and must use them sequentially.
// Bounds and position selection use checked runtime picking helpers, including
// compact, compound, and infinite domains.
func EvalNondetWithOptions(ctx context.Context, name string, storage *VariableStorage, rng *RNG,
	setEval func(context.Context) (RuntimeValue, error),
	bodyEval func(context.Context, RuntimeValue, error) (RuntimeValue, error),
	options RuntimeNondetOptions,
) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if storage == nil || rng == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Nondeterministic binding requires variable storage and RNG"}
	}
	if setEval == nil || bodyEval == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Nondeterministic binding requires set and body evaluators"}
	}
	// The compiler normally discovers all names up front. Also register a name
	// on direct use without clearing a value saved by an earlier invocation.
	if _, exists := storage.NondetPicks.Get2(name); !exists {
		storage.NondetPicks.Set(name, RuntimeValue{})
	}
	set, err := setEval(ctx)
	if err != nil {
		return RuntimeValue{}, err
	}
	if !set.Valid() {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	domain, err := RuntimeDomainFromValue(set)
	if err != nil {
		return RuntimeValue{}, err
	}
	cardinality, cardinalityErr := domain.Cardinality(ctx)
	if cardinalityErr == nil && cardinality.Big().Sign() == 0 {
		return NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	}
	if cardinalityErr != nil && !runtimeDomainIsDiagnostic(cardinalityErr) {
		return RuntimeValue{}, cardinalityErr
	}
	bounds, err := RuntimePickBoundsContext(ctx, set)
	if err != nil {
		return RuntimeValue{}, err
	}
	positions := make([]Integer, len(bounds))
	finiteBounds := make([]Integer, len(bounds))
	retry := true
	for i, bound := range bounds {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		limit := bound.Value
		if bound.Finite {
			if limit.Big().Sign() <= 0 {
				// The nondeterministic evaluator draws in bound order. Unlike
				// standalone sampling, an erroneous zero bound may follow draws
				// from earlier infinite factors and must retain their RNG state.
				return RuntimeValue{}, RuntimeDomainException{Name: "AssertionError", Message: "The expression evaluated to a falsy value:\n\n  (bound > 0n)\n"}
			}
			finiteBounds[i] = bound.Value
		} else {
			limit = IntegerFromBig(new(big.Int).Lsh(big.NewInt(1), 256))
			retry = false
		}
		positions[i], err = rng.Next(limit)
		if err != nil {
			return RuntimeValue{}, err
		}
		if !bound.Finite {
			position := positions[i].Big()
			position.Sub(position, new(big.Int).Lsh(big.NewInt(1), 255))
			positions[i] = IntegerFromBig(position)
		} else if bound.Value.Big().Cmp(options.RetrySmallerThan.Big()) >= 0 {
			retry = false
		}
	}
	original := append([]Integer(nil), positions...)
	for {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		picked, pickErr := PickRuntimePositionContext(ctx, set, positions)
		if pickErr != nil && !runtimeDomainIsDiagnostic(pickErr) {
			// A thrown pick exception occurs before the reference fills the
			// binding cache or calls its body. Returned diagnostics stay lazy.
			return RuntimeValue{}, pickErr
		}
		result, bodyErr := bodyEval(ctx, picked, pickErr)
		if bodyErr == nil && result.expression.Kind != "bool" {
			bodyErr = Diagnostic{Code: "QNT501", Message: "Expected a Boolean value"}
		}
		if bodyErr != nil || result.expression.Bool || !retry {
			if pickErr == nil {
				storage.NondetPicks.Set(name, picked)
			}
			return result, bodyErr
		}
		incrementRuntimePositions(positions, finiteBounds)
		exhausted := true
		for i, position := range positions {
			if position.Big().Cmp(original[i].Big()) != 0 {
				exhausted = false
				break
			}
		}
		if exhausted {
			// Upstream only records the pick when it stops retrying. Exhausting
			// a cycle leaves any previously recorded value unchanged.
			return result, nil
		}
	}
}

func incrementRuntimePositions(positions, bounds []Integer) {
	for i := len(positions) - 1; i >= 0; i-- {
		next := new(big.Int).Add(positions[i].Big(), big.NewInt(1))
		if next.Cmp(bounds[i].Big()) < 0 {
			positions[i] = IntegerFromBig(next)
			return
		}
		positions[i] = Integer{}
	}
}
