package quint

import (
	"context"
	"math"
	"math/big"
)

// EvalRangeBuiltin constructs the reference's exclusive-end list range.
// Endpoints use JavaScript Number rounding, and descending ranges step by -1.
func EvalRangeBuiltin(ctx context.Context, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if len(args) != 2 {
		return fail("range requires 2 integer arguments")
	}
	for _, arg := range args {
		if !arg.Valid() || arg.expression.Kind != "int" {
			return fail("range requires integer arguments")
		}
	}
	start, _ := args[0].expression.Int.Big().Float64()
	end, _ := args[1].expression.Int.Big().Float64()
	if math.IsInf(start, 0) || math.IsInf(end, 0) {
		return fail("Cannot materialize a range with infinite Number endpoints")
	}
	length := math.Abs(end - start)
	step := 1.0
	if end < start {
		step = -1
	}
	elements := []Expr{}
	current := start
	for i := 0.0; i < length; i++ {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		n, _ := new(big.Float).SetFloat64(current).Int(nil)
		elements = append(elements, Expr{Kind: "int", Int: IntegerFromBig(n)})
		current += step
	}
	return NewRuntimeValue(Expr{Kind: "app", Opcode: "List", Args: elements})
}
