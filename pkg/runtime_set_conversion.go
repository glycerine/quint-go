package quint

import (
	"fmt"
)

func setConversionBuiltinArity(op string) int {
	switch op {
	case "getOnlyElement":
		return 1
	}
	return 0
}

// EvalSetConversionBuiltin extracts the sole element of a finite explicit set.
// Results own their IR, retaining nested runtime source representations.
func EvalSetConversionBuiltin(op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if setConversionBuiltinArity(op) == 0 {
		return fail(fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != 1 {
		return fail(op + " requires 1 argument")
	}
	set := args[0]
	if !set.Valid() {
		return fail("Invalid runtime value")
	}
	if set.expression.Kind != "app" || set.expression.Opcode != "Set" {
		return fail(op + " requires a Set value")
	}
	if size := len(set.expression.Args); size != 1 {
		return RuntimeValue{}, Diagnostic{Code: "QNT505", Message: fmt.Sprintf("Called 'getOnlyElement' on a set with %d elements. Make sure the set has exactly one element.", size)}
	}
	// Set.add replaces the stored value for a structurally equal key.
	return NewRuntimeValue(set.source.Args[len(set.source.Args)-1])
}
