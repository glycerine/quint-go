package quint

import "fmt"

// EvalIntegerBuiltin evaluates the strict integer operators. Arguments have
// already been evaluated; arity/type failures are reported rather than panicking.
func EvalIntegerBuiltin(op string, args []RuntimeValue) (RuntimeValue, error) {
	arity := 2
	switch op {
	case "iuminus":
		arity = 1
	case "iadd", "isub", "imul", "idiv", "imod", "ipow", "ilt", "ilte", "igt", "igte":
	default:
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: fmt.Sprintf("Runtime does not support the built -in operator '%s'", op)}
	}
	if len(args) != arity {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: fmt.Sprintf("%s requires %d integer arguments", op, arity)}
	}
	for _, arg := range args {
		if !arg.Valid() || arg.expression.Kind != "int" {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: fmt.Sprintf("%s requires integer arguments", op)}
		}
	}
	a := args[0].expression.Int.Big()
	result := Expr{Kind: "int"}
	if arity == 1 {
		a.Neg(a)
	} else {
		b := args[1].expression.Int.Big()
		switch op {
		case "iadd":
			a.Add(a, b)
		case "isub":
			a.Sub(a, b)
		case "imul":
			a.Mul(a, b)
		case "idiv", "imod":
			if b.Sign() == 0 {
				return RuntimeValue{}, Diagnostic{Code: "QNT503", Message: "Division by zero"}
			}
			if op == "idiv" {
				a.Quo(a, b)
			} else {
				a.Rem(a, b)
			}
		case "ipow":
			if a.Sign() == 0 && b.Sign() == 0 {
				return RuntimeValue{}, Diagnostic{Code: "QNT503", Message: "0^0 is undefined"}
			}
			if b.Sign() < 0 {
				return RuntimeValue{}, Diagnostic{Code: "QNT503", Message: "i^j is undefined for j < 0"}
			}
			a.Exp(a, b, nil)
		default:
			cmp := a.Cmp(b)
			value := false
			switch op {
			case "ilt":
				value = cmp < 0
			case "ilte":
				value = cmp <= 0
			case "igt":
				value = cmp > 0
			case "igte":
				value = cmp >= 0
			}
			result = Expr{Kind: "bool", Bool: value}
		}
	}
	if result.Kind == "int" {
		result.Int = IntegerFromBig(a)
	}
	return NewRuntimeValue(result)
}
