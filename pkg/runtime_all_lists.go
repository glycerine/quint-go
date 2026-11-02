package quint

import (
	"context"
	"fmt"
	"math"
)

func runtimeAllListsBuiltinArity(op string) int {
	if op == "allListsUpTo" {
		return 2
	}
	return 0
}

type runtimeListArray struct {
	identity runtimeObjectIdentity
	values   []RuntimeValue
}

// EvalAllListsBuiltin materializes finite allListsUpTo using the reference's
// intermediate sets of JavaScript array objects. hashes belongs to the caller:
// its counter affects collision traversal across successive operations and is
// deliberately not an implicit process-global variable.
func EvalAllListsBuiltin(ctx context.Context, op string, args []RuntimeValue, hashes *RuntimeObjectHashState) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if runtimeAllListsBuiltinArity(op) == 0 {
		return fail(fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != 2 {
		return fail("allListsUpTo requires 2 arguments")
	}
	for _, value := range args {
		if !value.Valid() {
			return fail("Invalid runtime value")
		}
	}
	if args[0].expression.Kind != "app" || args[0].expression.Opcode != "Set" {
		return fail("allListsUpTo requires a Set value")
	}
	if args[1].expression.Kind != "int" {
		return fail("allListsUpTo requires an integer length")
	}
	if hashes == nil {
		return fail("allListsUpTo requires an object hash state")
	}
	entries, err := runtimeEntries(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	// lodash.times first coerces Number to an integer, returns [] outside the
	// safe-integer range, and exposes at most MAX_ARRAY_LENGTH iterations to
	// the caller's forEach. Integer input cannot have a fractional endpoint.
	length, _ := args[1].expression.Int.Big().Float64()
	if length < 1 || length > 9007199254740991 || math.IsInf(length, 0) {
		length = 0
	}
	if length > math.MaxUint32 {
		length = math.MaxUint32
	}
	var lists, last runtimeObjectSet[*runtimeListArray]
	empty, previousEmpty := &runtimeListArray{}, &runtimeListArray{}
	lists.Add(hashes, &empty.identity, empty)
	last.Add(hashes, &previousEmpty.identity, previousEmpty)
	for size := uint64(0); size < uint64(length) && len(entries) != 0; size++ {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		var next runtimeObjectSet[*runtimeListArray]
		for _, element := range entries {
			// Set.flatMap consumes each mapped Set before moving on to the
			// next domain value. Its inner Set.map hashes long results eagerly.
			var mapped runtimeObjectSet[*runtimeListArray]
			for _, old := range last.Entries() {
				if err := ctx.Err(); err != nil {
					return RuntimeValue{}, err
				}
				array := &runtimeListArray{values: append(append([]RuntimeValue{}, old.Value.values...), element.Value)}
				mapped.Add(hashes, &array.identity, array)
			}
			for _, generated := range mapped.Entries() {
				if err := ctx.Err(); err != nil {
					return RuntimeValue{}, err
				}
				next.Add(hashes, generated.Key, generated.Value)
			}
		}
		for _, generated := range next.Entries() {
			if err := ctx.Err(); err != nil {
				return RuntimeValue{}, err
			}
			lists.Add(hashes, generated.Key, generated.Value)
		}
		last = next
	}
	// Set.map first creates a transient Set of RuntimeValueList objects. The
	// subsequent toOrderedSet preserves that traversal for rv.mkSet's final
	// persistent normalized rebuild.
	results := make([]runtimeReferenceEntry, 0, len(lists.entries))
	for _, entry := range lists.Entries() {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		list, err := newRuntimeComposite("List", entry.Value.values)
		if err != nil {
			return RuntimeValue{}, err
		}
		results = append(results, runtimeReferenceEntry{Key: list, Value: list})
	}
	mapped, err := runtimeReferenceMapFromEntries(results)
	if err != nil {
		return RuntimeValue{}, err
	}
	values := make([]RuntimeValue, 0, mapped.Len())
	for _, entry := range mapped.Entries() {
		values = append(values, entry.Value)
	}
	return newRuntimeComposite("Set", values)
}
