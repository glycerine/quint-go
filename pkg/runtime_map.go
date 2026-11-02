package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
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

// EvalMapBuiltin implements strict map lookup/update and key enumeration.
// Keys compare structurally; duplicate keys retain their first insertion
// position and their last value, as in the reference's small immutable maps.
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
	pairs := insdict.NewDict[string, Expr]()
	for _, pair := range args[0].source.Args {
		key, err := NewRuntimeValue(pair.Args[0])
		if err != nil {
			return RuntimeValue{}, err
		}
		pairs.Set(key.identity, pair)
	}
	if op == "keys" {
		keys := []Expr{}
		for _, pair := range pairs.All() {
			keys = append(keys, pair.Args[0])
		}
		return NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: keys})
	}
	key := args[1]
	pair, found := pairs.Get2(key.identity)
	if op == "get" {
		if found {
			return NewRuntimeValue(pair.Args[1])
		}
		names := []string{}
		for _, entry := range pairs.All() {
			value, err := NewRuntimeValue(entry.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			names = append(names, value.String())
		}
		return fail("QNT507", fmt.Sprintf("Called 'get' with a non-existing key. Key is %s. Map has keys: %s", key.String(), strings.Join(names, ", ")))
	}
	if op == "set" && !found {
		return fail("QNT507", "Called 'set' with a non-existing key")
	}
	keyExpr := key.source
	if found {
		keyExpr = pair.Args[0]
	}
	pairs.Set(key.identity, Expr{Kind: "app", Opcode: "Tup", Args: []Expr{keyExpr, args[2].source}})
	entries := []Expr{}
	for _, entry := range pairs.All() {
		entries = append(entries, entry)
	}
	return NewRuntimeValue(Expr{Kind: "app", Opcode: "Map", Args: entries})
}
