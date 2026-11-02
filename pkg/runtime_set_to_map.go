package quint

import "context"

// EvalSetToMapBuiltin converts a concrete set of two-element tuples or lists to
// a map. The intermediate Set of fresh pair arrays uses object identity hashes;
// hashes must be shared across calls to reproduce one reference process.
func EvalSetToMapBuiltin(ctx context.Context, args []RuntimeValue, hashes *RuntimeObjectHashState) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if len(args) != 1 {
		return fail("setToMap requires 1 argument")
	}
	set := args[0]
	if !set.Valid() {
		return fail("Invalid runtime value")
	}
	if set.expression.Kind != "app" || set.expression.Opcode != "Set" {
		return fail("setToMap requires a Set value")
	}
	if hashes == nil {
		return fail("setToMap requires an object hash state")
	}
	entries, err := runtimeEntries(set)
	if err != nil {
		return RuntimeValue{}, err
	}
	var pairs runtimeObjectSet[runtimeReferenceEntry]
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		pair := entry.Value
		if pair.expression.Kind != "app" || (pair.expression.Opcode != "Tup" && pair.expression.Opcode != "List") {
			return fail("Expected a 2-tuple")
		}
		children, err := runtimeChildren(pair)
		if err != nil {
			return RuntimeValue{}, err
		}
		if len(children) != 2 {
			return fail("Expected a 2-tuple")
		}
		// toTuple2 creates a fresh JavaScript array for every input element.
		pairs.Add(hashes, &runtimeObjectIdentity{}, runtimeReferenceEntry{Key: children[0], Value: children[1]})
	}
	raw := make([]runtimeReferenceEntry, 0, len(entries))
	for _, pair := range pairs.Entries() {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		raw = append(raw, pair.Value)
	}
	// Map(set.map(toTuple2)) resolves duplicates before rv.mkMap normalizes
	// its surviving keys and constructs a second transient map.
	intermediate, err := runtimeReferenceMapFromEntries(raw)
	if err != nil {
		return RuntimeValue{}, err
	}
	normalized := intermediate.Entries()
	for i, entry := range normalized {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		normalized[i].Key, err = runtimeNormalForm(entry.Key)
		if err != nil {
			return RuntimeValue{}, err
		}
	}
	result, err := runtimeReferenceMapFromEntries(normalized)
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeWithEntries("Map", result)
}
