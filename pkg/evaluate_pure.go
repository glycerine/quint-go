package quint

import (
	"context"
	"fmt"
	"math/big"

	"github.com/glycerine/insdict"
)

// EvaluatePureExpression evaluates concrete constructors, strict integer/Boolean
// operations, and lazy Boolean/conditional expressions. Bindings contain already
// evaluated values. Local value definitions are lazy and memoized per scope.
// Module definition lookup and stateful operations belong
// to the full evaluator and are not yet handled here.
func EvaluatePureExpression(ctx context.Context, expression Expr, bindings *insdict.Dict[string, RuntimeValue]) (RuntimeValue, error) {
	return evaluateExpression(ctx, expression, bindings, nil, nil)
}

// EvaluateStateExpression adds registered variable reads and assignments to the
// pure evaluator. Assignments update next registers; callers explicitly shift
// or recover storage when composing actions.
func EvaluateStateExpression(ctx context.Context, expression Expr, storage *VariableStorage) (RuntimeValue, error) {
	if storage == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "State evaluation requires variable storage"}
	}
	return evaluateExpression(ctx, expression, nil, storage, nil)
}

// EvaluateStateExpressionWithRNG enables randomized action choice with caller-owned
// state and RNG. Both retain changes/counter advancement across invocations.
func EvaluateStateExpressionWithRNG(ctx context.Context, expression Expr, storage *VariableStorage, rng *RNG) (RuntimeValue, error) {
	if storage == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "State evaluation requires variable storage"}
	}
	return evaluateExpression(ctx, expression, nil, storage, rng)
}

func evaluateExpression(ctx context.Context, expression Expr, bindings *insdict.Dict[string, RuntimeValue], storage *VariableStorage, rng *RNG, traces ...*RuntimeTrace) (RuntimeValue, error) {
	if bindings != nil {
		owned := insdict.NewDict[string, RuntimeValue]()
		for name, value := range bindings.All() {
			owned.Set(name, value)
		}
		bindings = owned
	}
	var trace *RuntimeTrace
	if len(traces) > 0 {
		trace = traces[0]
	}
	shift := func() error {
		if trace != nil {
			return trace.Shift(storage)
		}
		storage.Shift()
		storage.ClearMetadata()
		return nil
	}
	var eval func(Expr) (RuntimeValue, error)
	type localValue func([]RuntimeValue) (RuntimeValue, error)
	scopes := []*insdict.Dict[string, localValue]{}
	lookup := func(name string) (localValue, bool) {
		for i := len(scopes) - 1; i >= 0; i-- {
			if value, ok := scopes[i].Get2(name); ok {
				return value, true
			}
		}
		return nil, false
	}
	fail := func(e Expr, message string) (RuntimeValue, error) {
		id := e.ID
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message, Reference: &id}
	}
	boolValue := func(b bool) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: b}) }
	eval = func(e Expr) (RuntimeValue, error) {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		switch e.Kind {
		case "int", "bool", "str":
			return NewRuntimeValue(e)
		case "name":
			if value, ok := lookup(e.Name); ok {
				return value(nil)
			}
			if bindings != nil {
				if value, ok := bindings.Get2(e.Name); ok && value.Valid() {
					return value, nil
				}
			}
			if storage != nil {
				for _, register := range storage.current.All() {
					if register.Name == e.Name {
						value, err := register.Read()
						if d, ok := err.(Diagnostic); ok {
							id := e.ID
							d.Reference = &id
							err = d
						}
						return value, err
					}
				}
			}
			if e.Name == "Bool" {
				return NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "bool", Bool: false}, {Kind: "bool", Bool: true}}})
			}
			if e.Name == "q::lastTrace" {
				args := []Expr{}
				if trace != nil {
					for _, state := range trace.States() {
						args = append(args, copyRuntimeExpression(state.source))
					}
				}
				return NewRuntimeValue(Expr{Kind: "app", Opcode: "List", Args: args})
			}
			return fail(e, "Unbound runtime name: "+e.Name)
		case "lambda":
			e = copyRuntimeExpression(e)
			if e.Body == nil {
				return fail(e, "Malformed local operator")
			}
			captured := append([]*insdict.Dict[string, localValue](nil), scopes...)
			return newRuntimeClosure(len(e.Params), func(callContext context.Context, arguments []RuntimeValue) (RuntimeValue, error) {
				if len(arguments) != len(e.Params) {
					return fail(e, fmt.Sprintf("lambda requires %d arguments", len(e.Params)))
				}
				parameters := insdict.NewDict[string, localValue]()
				for i, param := range e.Params {
					value := arguments[i]
					parameters.Set(param.Name, func(extra []RuntimeValue) (RuntimeValue, error) {
						if extra != nil {
							return value.Apply(ctx, extra)
						}
						return value, nil
					})
				}
				saved, savedContext := scopes, ctx
				scopes = append(append([]*insdict.Dict[string, localValue](nil), captured...), parameters)
				ctx = callContext
				result, err := eval(*e.Body)
				scopes, ctx = saved, savedContext
				return result, err
			}), nil

		case "let":
			if e.Opdef == nil || e.Opdef.Expr == nil || e.Body == nil {
				return fail(e, "Malformed local definition")
			}
			if e.Opdef.Qualifier == Nondet {
				return fail(e, "Local nondeterministic definitions require the full evaluator")
			}
			captured := append([]*insdict.Dict[string, localValue](nil), scopes...)
			var cached RuntimeValue
			var cachedErr error
			evaluated := false
			var generation uint64
			thunk := func(arguments []RuntimeValue) (RuntimeValue, error) {

				if storage != nil && e.Opdef.Qualifier == Val && e.Opdef.Depth != nil && *e.Opdef.Depth == 0 && generation != storage.generation {
					evaluated = false
				}
				if !evaluated || (e.Opdef.Qualifier == Action && e.Opdef.Expr.Kind != "lambda") {
					if storage != nil && e.Opdef.Qualifier == Action && e.Opdef.Expr.Kind != "lambda" && storage.ActionTaken == "" {
						storage.ActionTaken = e.Opdef.Name
					}
					saved := scopes
					scopes = captured
					cached, cachedErr = eval(*e.Opdef.Expr)
					if cachedErr == nil && storage != nil && e.Opdef.Qualifier == Action && cached.closure != nil {
						original := cached
						cached = newRuntimeClosure(len(e.Opdef.Expr.Params), func(callContext context.Context, values []RuntimeValue) (RuntimeValue, error) {
							if storage.ActionTaken == "" {
								storage.ActionTaken = e.Opdef.Name
							}
							return original.Apply(callContext, values)
						})
					}
					scopes = saved
					evaluated = true
					if storage != nil {
						generation = storage.generation
					}
				}
				if cachedErr != nil {
					return RuntimeValue{}, cachedErr
				}
				if arguments != nil {
					result, err := cached.Apply(ctx, arguments)
					if d, ok := err.(Diagnostic); ok && d.Reference == nil {
						id := e.ID
						d.Reference = &id
						err = d
					}
					return result, err
				}
				return cached, nil
			}
			scope := insdict.NewDict[string, localValue]()
			scope.Set(e.Opdef.Name, thunk)
			scopes = append(scopes, scope)
			result, err := eval(*e.Body)
			scopes = scopes[:len(scopes)-1]
			return result, err
		case "app":
			if value, ok := lookup(e.Opcode); ok {
				arguments := make([]RuntimeValue, len(e.Args))
				for i, child := range e.Args {
					argument, err := eval(child)
					if err != nil {
						return RuntimeValue{}, err
					}
					arguments[i] = argument
				}
				return value(arguments)
			}
		default:
			return fail(e, "Unsupported pure expression kind: "+e.Kind)
		}
		boolean := func(child Expr) (bool, error) {
			v, err := eval(child)
			if err != nil {
				return false, err
			}
			if v.expression.Kind != "bool" {
				_, err = fail(child, "Expected a Boolean value")
				return false, err
			}
			return v.expression.Bool, nil
		}
		switch e.Opcode {
		case "next", "oneOf":
			id := e.ID
			return RuntimeValue{}, Diagnostic{Code: "QNT000", Message: "Unknown stateful op", Reference: &id}
		case "then":
			if storage == nil {
				return fail(e, "Action sequencing requires variable storage")
			}
			if len(e.Args) != 2 {
				return fail(e, "then requires 2 arguments")
			}
			enabled, err := boolean(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if !enabled {
				id := e.Args[0].ID
				if e.Args[0].Kind == "app" && e.Args[0].Opcode == "then" && len(e.Args[0].Args) == 2 {
					id = e.Args[0].Args[1].ID
				}
				return RuntimeValue{}, Diagnostic{Code: "QNT513", Message: "Cannot continue in `then` because the highlighted expression evaluated to false", Reference: &id}
			}
			if err := shift(); err != nil {
				return RuntimeValue{}, err
			}
			return eval(e.Args[1])
		case "reps":
			if storage == nil {
				return fail(e, "Action repetition requires variable storage")
			}
			if len(e.Args) != 2 {
				return fail(e, "reps requires 2 arguments")
			}
			count, err := eval(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if count.expression.Kind != "int" {
				return fail(e, "reps requires an integer count")
			}
			limit, _ := count.expression.Int.Big().Float64()
			result, _ := boolValue(true)
			for i := float64(0); i < limit; i++ {
				if err := ctx.Err(); err != nil {
					return RuntimeValue{}, err
				}
				callback, err := eval(e.Args[1])
				if err != nil {
					return RuntimeValue{}, err
				}
				integer, _ := new(big.Float).SetFloat64(i).Int(nil)
				n := IntegerFromBig(integer)
				iteration, err := NewRuntimeValue(Expr{Kind: "int", Int: n})
				if err != nil {
					return RuntimeValue{}, err
				}
				result, err = callback.Apply(ctx, []RuntimeValue{iteration})
				if err != nil {
					return RuntimeValue{}, err
				}
				if result.expression.Kind != "bool" {
					return fail(e, "Expected a Boolean action result")
				}
				if !result.expression.Bool {
					id := e.ID
					return RuntimeValue{}, Diagnostic{Code: "QNT513", Message: fmt.Sprintf("Reps loop could not continue after iteration #%s evaluated to false", runtimeNumberString(i+1)), Reference: &id}
				}
				if i < limit-1 {
					if err := shift(); err != nil {
						return RuntimeValue{}, err
					}
				}
			}
			return result, nil
		case "actionAny":
			if storage == nil || rng == nil {
				return fail(e, "Action choice requires variable storage and RNG")
			}
			snapshot := storage.Snapshot()
			indices := make([]int, len(e.Args))
			for i := range indices {
				indices[i] = i
			}
			for i := len(indices) - 1; i > 0; i-- {
				bound, _ := ParseInteger(fmt.Sprint(i + 1))
				draw, err := rng.Next(bound)
				if err != nil {
					return RuntimeValue{}, err
				}
				j := int(draw.Big().Int64())
				indices[i], indices[j] = indices[j], indices[i]
			}
			for _, index := range indices {
				storage.ActionTaken = ""
				for name := range storage.NondetPicks.All() {
					storage.NondetPicks.Set(name, RuntimeValue{})
				}
				enabled, err := boolean(e.Args[index])
				if err != nil {
					return RuntimeValue{}, err
				}
				if enabled {
					storage.Recover(storage.Snapshot())
					return boolValue(true)
				}
				storage.Recover(snapshot)
			}
			storage.Recover(snapshot)
			return boolValue(false)
		case "actionAll":
			if storage == nil {
				return fail(e, "Action conjunction requires variable storage")
			}
			snapshot := storage.Snapshot()
			for _, action := range e.Args {
				enabled, err := boolean(action)
				if err != nil {
					return RuntimeValue{}, err
				}
				if !enabled {
					storage.Recover(snapshot)
					return boolValue(false)
				}
			}
			return boolValue(true)
		case "expect":
			if storage == nil {
				return fail(e, "Expect requires variable storage")
			}
			if len(e.Args) != 2 {
				return fail(e, "Expect requires an action and condition")
			}
			enabled, err := boolean(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if !enabled {
				return RuntimeValue{}, Diagnostic{Code: "QNT508", Message: "Cannot continue to \"expect\"", Reference: &e.ID}
			}
			snapshot := storage.Snapshot()
			if err := shift(); err != nil {
				return RuntimeValue{}, err
			}
			condition, err := boolean(e.Args[1])
			if err != nil {
				return RuntimeValue{}, err
			}
			storage.Recover(snapshot)
			if trace != nil {
				trace.DropLast()
			}
			if !condition {
				return RuntimeValue{}, Diagnostic{Code: "QNT508", Message: "Expect condition does not hold true", Reference: &e.ID}
			}
			return boolValue(true)
		case "assign":
			if storage == nil {
				return fail(e, "Assignment requires variable storage")
			}
			if len(e.Args) != 2 || e.Args[0].Kind != "name" {
				return fail(e, "Assignment requires a variable and value")
			}
			if _, local := lookup(e.Args[0].Name); local {
				return fail(e, "Assignment target is a local binding")
			}
			for key, register := range storage.current.All() {
				if register.Name == e.Args[0].Name {
					value, err := eval(e.Args[1])
					if err != nil {
						return RuntimeValue{}, err
					}
					storage.next.Get(key).Value = value
					return boolValue(true)
				}
			}
			return fail(e, "Unknown assignment variable: "+e.Args[0].Name)
		case "matchVariant":
			if len(e.Args) < 1 || len(e.Args)%2 != 1 {
				return fail(e, "matchVariant requires a value and label/branch pairs")
			}
			matched, err := eval(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if matched.expression.Kind != "app" || matched.expression.Opcode != "variant" {
				return fail(e, "Expected a variant value")
			}
			label := matched.expression.Args[0].Str
			for i := 1; i < len(e.Args); i += 2 {
				candidate, err := eval(e.Args[i])
				if err != nil {
					return RuntimeValue{}, err
				}
				if candidate.expression.Kind != "str" {
					return fail(e.Args[i], "Expected a string variant label")
				}
				if candidate.expression.Str != "_" && candidate.expression.Str != label {
					continue
				}
				branch, err := eval(e.Args[i+1])
				if err != nil {
					return RuntimeValue{}, err
				}
				payload, err := NewRuntimeValue(matched.source.Args[1])
				if err != nil {
					return RuntimeValue{}, err
				}
				result, err := branch.Apply(ctx, []RuntimeValue{payload})
				if d, ok := err.(Diagnostic); ok && d.Reference == nil {
					id := e.ID
					d.Reference = &id
					err = d
				}
				return result, err
			}
			id := e.ID
			return RuntimeValue{}, Diagnostic{Code: "QNT505", Message: "No match for variant " + label, Reference: &id}
		case "and", "or":
			for _, child := range e.Args {
				b, err := boolean(child)
				if err != nil {
					return RuntimeValue{}, err
				}
				if b == (e.Opcode == "or") {
					return boolValue(b)
				}
			}
			return boolValue(e.Opcode == "and")
		case "ite", "implies":
			arity := 3
			if e.Opcode == "implies" {
				arity = 2
			}
			if len(e.Args) != arity {
				return fail(e, fmt.Sprintf("%s requires %d arguments", e.Opcode, arity))
			}
			b, err := boolean(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if e.Opcode == "implies" {
				if !b {
					return boolValue(true)
				}
				value, err := boolean(e.Args[1])
				if err != nil {
					return RuntimeValue{}, err
				}
				return boolValue(value)
			}
			if b {
				return eval(e.Args[1])
			}
			return eval(e.Args[2])
		}
		args := make([]RuntimeValue, len(e.Args))
		for i, child := range e.Args {
			value, err := eval(child)
			if err != nil {
				return RuntimeValue{}, err
			}
			args[i] = value
		}
		switch e.Opcode {
		case "List", "Tup", "Set", "Map", "Rec", "variant":
			value := Expr{Kind: "app", Opcode: e.Opcode, Args: make([]Expr, len(args))}
			for i, arg := range args {
				value.Args[i] = arg.source
			}
			return NewRuntimeValue(value)
		case "not", "fail", "iff", "eq", "neq", "assert":
			arity := 2
			if e.Opcode == "not" || e.Opcode == "fail" || e.Opcode == "assert" {
				arity = 1
			}
			if len(args) != arity {
				return fail(e, fmt.Sprintf("%s requires %d arguments", e.Opcode, arity))
			}
			if e.Opcode == "eq" || e.Opcode == "neq" {
				equal := args[0].Equal(args[1])
				return boolValue(equal == (e.Opcode == "eq"))
			}
			for _, arg := range args {
				if arg.expression.Kind != "bool" {
					return fail(e, "Expected a Boolean value")
				}
			}
			switch e.Opcode {
			case "not", "fail":
				return boolValue(!args[0].expression.Bool)
			case "iff":
				return boolValue(args[0].expression.Bool == args[1].expression.Bool)
			case "assert":
				if !args[0].expression.Bool {
					id := e.ID
					return RuntimeValue{}, Diagnostic{Code: "QNT508", Message: "Assertion failed", Reference: &id}
				}
				return args[0], nil
			}
		}
		var value RuntimeValue
		var err error
		if e.Opcode == "setBy" {
			value, err = EvalMapSetBy(ctx, args)
		} else if listHigherBuiltinArity(e.Opcode) != 0 {
			value, err = EvalListHigherBuiltin(ctx, e.Opcode, args)
		} else if e.Opcode == "range" {
			value, err = EvalRangeBuiltin(ctx, args)
		} else if sequenceBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSequenceBuiltin(e.Opcode, args)
		} else if mapBuiltinArity(e.Opcode) != 0 {
			value, err = EvalMapBuiltin(e.Opcode, args)
		} else if setConversionBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSetConversionBuiltin(e.Opcode, args)
		} else if setBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSetBuiltin(e.Opcode, args)
		} else {
			if e.Opcode == "apalache::generate" || !IsBuiltinOpcode(e.Opcode) {
				err = Diagnostic{Code: "QNT000", Message: "Unknown builtin " + e.Opcode}
			} else {
				value, err = EvalIntegerBuiltin(e.Opcode, args)
			}
		}
		if d, ok := err.(Diagnostic); ok {
			if d.Reference == nil {
				id := e.ID
				d.Reference = &id
			}
			err = d
		}
		return value, err
	}
	return eval(expression)
}
