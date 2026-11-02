package quint

import "fmt"

func setBuiltinArity(op string) int {
	switch op {
	case "size", "isFinite", "flatten":
		return 1
	case "contains", "in", "subseteq", "exclude", "union", "intersect":
		return 2
	}
	return 0
}

// EvalSetBuiltin evaluates operations on explicit finite sets using retained
// runtime entries. Intermediate set mutations retain their reference traversal;
// final results follow the reference's mkSet normalization/rebuild boundary.
func EvalSetBuiltin(op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	arity := setBuiltinArity(op)
	if arity == 0 {
		return fail(fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != arity {
		return fail(fmt.Sprintf("%s requires %d arguments", op, arity))
	}
	for _, arg := range args {
		if !arg.Valid() {
			return fail("Invalid runtime value")
		}
	}
	if op == "in" {
		args = []RuntimeValue{args[1], args[0]}
	}
	isSet := func(v RuntimeValue) bool { return v.expression.Kind == "app" && v.expression.Opcode == "Set" }
	if !isSet(args[0]) {
		return fail(op + " requires a Set value")
	}
	boolean := func(value bool) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: value}) }
	left, err := runtimeMap(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "size" {
		n, _ := ParseInteger(fmt.Sprint(left.Len()))
		return NewRuntimeValue(Expr{Kind: "int", Int: n})
	}
	if op == "isFinite" {
		return boolean(true)
	}
	if op == "contains" || op == "in" {
		key, err := runtimeNormalForm(args[1])
		if err != nil {
			return RuntimeValue{}, err
		}
		_, present, err := left.Lookup(key)
		if err != nil {
			return RuntimeValue{}, err
		}
		return boolean(present)
	}
	if op == "flatten" {
		// Set.map(s => s.toSet()) rebuilds an intermediate set keyed by the
		// underlying sets. Their structural hashes equal the wrapper hashes,
		// but constructor traversal can differ from the outer set's history.
		mapped := make([]runtimeReferenceEntry, 0, left.Len())
		for _, entry := range left.Entries() {
			if !isSet(entry.Value) {
				return fail("flatten requires a Set of Set values")
			}
			mapped = append(mapped, runtimeReferenceEntry{Key: entry.Value, Value: entry.Value})
		}
		intermediate, err := runtimeReferenceMapFromEntries(mapped)
		if err != nil {
			return RuntimeValue{}, err
		}
		// Immutable.Set.flatten builds its result through a mutable constructor
		// before rv.mkSet performs the final persistent normalized rebuild.
		flatEntries := []runtimeReferenceEntry{}
		for _, entry := range intermediate.Entries() {
			inner, err := runtimeEntries(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			for _, child := range inner {
				flatEntries = append(flatEntries, runtimeReferenceEntry{Key: child.Value, Value: child.Value})
			}
		}
		flat, err := runtimeReferenceMapFromEntries(flatEntries)
		if err != nil {
			return RuntimeValue{}, err
		}
		values := make([]RuntimeValue, 0, flat.Len())
		for _, entry := range flat.Entries() {
			values = append(values, entry.Value)
		}
		return newRuntimeComposite("Set", values)
	}
	if !isSet(args[1]) {
		return fail(op + " requires Set values")
	}
	right, err := runtimeMap(args[1])
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "subseteq" {
		for _, entry := range left.Entries() {
			_, present, err := right.Lookup(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			if !present {
				return boolean(false)
			}
		}
		return boolean(true)
	}
	output := left
	if op == "union" {
		switch {
		case right.Len() == 0:
		case left.Len() == 0:
			output = right
		default:
			output, err = runtimeReferenceMapBatchSet(left, right.Entries())
			if err != nil {
				return RuntimeValue{}, err
			}
		}
	} else {
		// Collect removal keys before touching the tree: small-map/collision
		// deletion swaps with the last entry and changes subsequent traversal.
		removed := []RuntimeValue{}
		for _, entry := range left.Entries() {
			_, present, err := right.Lookup(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			if (op == "exclude" && present) || (op == "intersect" && !present) {
				removed = append(removed, entry.Value)
			}
		}
		for _, key := range removed {
			output, err = output.Delete(key)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
	}
	values := make([]RuntimeValue, 0, output.Len())
	for _, entry := range output.Entries() {
		values = append(values, entry.Value)
	}
	return newRuntimeComposite("Set", values)
}
