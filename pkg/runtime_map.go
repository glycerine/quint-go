package quint

import (
	"fmt"
	"strings"
)

func mapBuiltinArity(op string) int {
	switch op {
	case "keys":
		return 1
	case "get":
		return 2
	case "set", "put":
		return 3
	}
	return 0
}

// EvalMapBuiltin implements strict map lookup/update and key enumeration over
// retained runtime entries. Updates preserve persistent traversal history and
// returning stored values retains their nested collection payloads.
func EvalMapBuiltin(op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(code, message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: code, Message: message}
	}
	arity := mapBuiltinArity(op)
	if arity == 0 {
		return fail("QNT501", fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != arity {
		return fail("QNT501", fmt.Sprintf("%s requires %d arguments", op, arity))
	}
	for _, arg := range args {
		if !arg.Valid() {
			return fail("QNT501", "Invalid runtime value")
		}
	}
	if args[0].expression.Kind != "app" || args[0].expression.Opcode != "Map" {
		return fail("QNT501", op+" requires a Map value")
	}
	entries, err := runtimeMap(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "keys" {
		keys := make([]RuntimeValue, 0, entries.Len())
		for _, entry := range entries.Entries() {
			keys = append(keys, entry.Key)
		}
		return newRuntimeComposite("Set", keys)
	}
	key, err := runtimeNormalForm(args[1])
	if err != nil {
		return RuntimeValue{}, err
	}
	value, found, err := entries.Lookup(key)
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "get" {
		if found {
			return value, nil
		}
		// The reference constructs the diagnostic view with Map.toMap(),
		// which rebuilds a map from its keyed sequence before enumerating keys.
		diagnosticEntries, err := runtimeReferenceMapFromEntries(entries.Entries())
		if err != nil {
			return RuntimeValue{}, err
		}
		names := make([]string, 0, entries.Len())
		for _, entry := range diagnosticEntries.Entries() {
			names = append(names, entry.Key.String())
		}
		return fail("QNT507", fmt.Sprintf("Called 'get' with a non-existing key. Key is %s. Map has keys: %s", key.String(), strings.Join(names, ", ")))
	}
	if op == "set" && !found {
		return fail("QNT507", "Called 'set' with a non-existing key")
	}
	updated, err := entries.Set(key, args[2])
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeWithEntries("Map", updated)
}
