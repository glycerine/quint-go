package quint

import (
	"context"
	"fmt"

	"github.com/glycerine/insdict"
)

// EvaluatePureExpression evaluates concrete constructors, strict integer/Boolean
// operations, and lazy Boolean/conditional expressions. Bindings contain already
// evaluated values. Definition lookup, closures, and stateful operations belong
// to the full evaluator and are not yet handled here.
func EvaluatePureExpression(ctx context.Context, expression Expr, bindings *insdict.Dict[string, RuntimeValue]) (RuntimeValue, error) {
	var eval func(Expr) (RuntimeValue, error)
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
			if bindings != nil {
				if value, ok := bindings.Get2(e.Name); ok && value.Valid() {
					return value, nil
				}
			}
			return fail(e, "Unbound runtime name: "+e.Name)
		case "app":
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
				value.Args[i] = arg.expression
			}
			return NewRuntimeValue(value)
		case "not", "iff", "eq", "neq", "assert":
			arity := 2
			if e.Opcode == "not" || e.Opcode == "assert" {
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
			case "not":
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
		if sequenceBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSequenceBuiltin(e.Opcode, args)
		} else {
			value, err = EvalIntegerBuiltin(e.Opcode, args)
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
