package quint

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func runtimeContextSetLike(value RuntimeValue) bool {
	return value.domain != nil || (value.expression.Kind == "app" && value.expression.Opcode == "Set")
}

// EvalSetBuiltinContext applies predicates directly to compact domains and
// materializes only at the reference's explicit toSet operation boundaries.
func EvalSetBuiltinContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if !runtimeHasDomain(args) {
		return EvalSetBuiltin(op, args)
	}
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
	for _, value := range args {
		if !value.Valid() {
			return fail("Invalid runtime value")
		}
	}
	if op == "in" {
		args = []RuntimeValue{args[1], args[0]}
	}
	if !runtimeContextSetLike(args[0]) {
		return fail(op + " requires a Set value")
	}
	boolean := func(value bool) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: value}) }
	leftDomain, err := RuntimeDomainFromValue(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	switch op {
	case "isFinite":
		// The reference returns true even for its Int and Nat runtime values.
		return boolean(true)
	case "size":
		size, err := leftDomain.Cardinality(ctx)
		if err != nil {
			return RuntimeValue{}, err
		}
		return NewRuntimeValue(Expr{Kind: "int", Int: size})
	case "contains", "in":
		var present bool
		if args[1].domain != nil {
			present, err = leftDomain.ContainsDomain(ctx, *args[1].domain)
		} else {
			present, err = leftDomain.Contains(ctx, args[1])
		}
		if err != nil {
			return RuntimeValue{}, err
		}
		return boolean(present)
	case "subseteq":
		if !runtimeContextSetLike(args[1]) {
			return fail(op + " requires Set values")
		}
		right, err := RuntimeDomainFromValue(args[1])
		if err != nil {
			return RuntimeValue{}, err
		}
		contained, err := leftDomain.Subset(ctx, right)
		if err != nil {
			return RuntimeValue{}, err
		}
		return boolean(contained)
	}
	leftValue, err := runtimeToSetContext(ctx, args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	left, err := runtimeMap(leftValue)
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "flatten" {
		mapped := make([]runtimeReferenceEntry, 0, left.Len())
		for _, entry := range left.Entries() {
			if err := ctx.Err(); err != nil {
				return RuntimeValue{}, err
			}
			if !runtimeContextSetLike(entry.Value) {
				return fail("flatten requires a Set of Set values")
			}
			inner, err := runtimeToSetContext(ctx, entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			mapped = append(mapped, runtimeReferenceEntry{Key: inner, Value: inner})
		}
		intermediate, err := runtimeRawMapFromEntriesContext(ctx, mapped)
		if err != nil {
			return RuntimeValue{}, err
		}
		flatEntries := []runtimeReferenceEntry{}
		for _, entry := range intermediate.Entries() {
			inner, err := runtimeEntries(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			for _, child := range inner {
				if err := ctx.Err(); err != nil {
					return RuntimeValue{}, err
				}
				flatEntries = append(flatEntries, runtimeReferenceEntry{Key: child.Value, Value: child.Value})
			}
		}
		flat, err := runtimeRawMapFromEntriesContext(ctx, flatEntries)
		if err != nil {
			return RuntimeValue{}, err
		}
		return runtimeContextSetFromEntries(ctx, flat)
	}
	if !runtimeContextSetLike(args[1]) {
		return fail(op + " requires Set values")
	}
	rightValue, err := runtimeToSetContext(ctx, args[1])
	if err != nil {
		return RuntimeValue{}, err
	}
	right, err := runtimeMap(rightValue)
	if err != nil {
		return RuntimeValue{}, err
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
		removed := []RuntimeValue{}
		for _, entry := range left.Entries() {
			if err := ctx.Err(); err != nil {
				return RuntimeValue{}, err
			}
			_, present, err := right.Lookup(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			if (op == "exclude" && present) || (op == "intersect" && !present) {
				removed = append(removed, entry.Value)
			}
		}
		for _, key := range removed {
			if err := ctx.Err(); err != nil {
				return RuntimeValue{}, err
			}
			output, err = output.Delete(key)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
	}
	return runtimeContextSetFromEntries(ctx, output)
}

func runtimeContextSetFromEntries(ctx context.Context, entries runtimeReferenceMap) (RuntimeValue, error) {
	values := make([]RuntimeValue, 0, entries.Len())
	for _, entry := range entries.Entries() {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		values = append(values, entry.Value)
	}
	return NewRuntimeCompositeContext(ctx, "Set", values)
}

// EvalSetHigherBuiltinContext keeps callbacks and their results context-aware,
// including when a concrete input set's callback returns a compact domain.
func EvalSetHigherBuiltinContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	arity := setHigherBuiltinArity(op)
	if arity == 0 {
		return fail(fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != arity {
		return fail(fmt.Sprintf("%s requires %d arguments", op, arity))
	}
	for _, value := range args {
		if !value.Valid() {
			return fail("Invalid runtime value")
		}
	}
	if !runtimeContextSetLike(args[0]) {
		return fail(op + " requires a Set value")
	}
	lambda := args[arity-1]
	// applyLambdaToSet and mapBy resolve their arrow before converting the
	// receiver. filter converts first, and fold resolves it only on reduction.
	if lambda.closure == nil && (op == "exists" || op == "forall" || op == "map" || op == "mapBy") {
		return fail("Expected a lambda value")
	}
	set, err := runtimeToSetContext(ctx, args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	entries, err := runtimeEntries(set)
	if err != nil {
		return RuntimeValue{}, err
	}
	if lambda.closure == nil && (op != "fold" || len(entries) > 0) {
		return fail("Expected a lambda value")
	}
	if op == "fold" {
		accumulator := args[1]
		for _, entry := range entries {
			accumulator, err = lambda.Apply(ctx, []RuntimeValue{accumulator, entry.Value})
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return accumulator, nil
	}
	results := make([]runtimeReferenceEntry, 0, len(entries))
	selected := make([]RuntimeValue, 0, len(entries))
	for _, entry := range entries {
		value, err := lambda.Apply(ctx, []RuntimeValue{entry.Value})
		if err != nil {
			return RuntimeValue{}, err
		}
		switch op {
		case "filter":
			if value.expression.Kind != "bool" {
				return fail("Expected a Boolean value")
			}
			if value.expression.Bool {
				selected = append(selected, entry.Value)
			}
		case "mapBy":
			key, err := runtimeKeyNormalFormContext(ctx, entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			results = append(results, runtimeReferenceEntry{Key: key, Value: value})
		default:
			results = append(results, runtimeReferenceEntry{Key: value, Value: value})
		}
	}
	if op == "filter" {
		return NewRuntimeCompositeContext(ctx, "Set", selected)
	}
	resultSet, err := runtimeRawMapFromEntriesContext(ctx, results)
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "mapBy" {
		return runtimeWithEntriesContext(ctx, "Map", resultSet)
	}
	if op == "map" {
		return runtimeContextSetFromEntries(ctx, resultSet)
	}
	for _, entry := range resultSet.Entries() {
		if entry.Value.expression.Kind != "bool" {
			return fail("Expected a Boolean value")
		}
		if (op == "exists" && entry.Value.expression.Bool) || (op == "forall" && !entry.Value.expression.Bool) {
			return NewRuntimeValue(Expr{Kind: "bool", Bool: op == "exists"})
		}
	}
	return NewRuntimeValue(Expr{Kind: "bool", Bool: op == "forall"})
}

// runtimeRawMapFromEntriesContext retains transient construction before the
// final mkSet/mkMap normalization. Mixed compact representations require a
// contextual HAMT equality path; reject those explicitly instead of comparing
// their compact display identities as though they were extensional values.
func runtimeRawMapFromEntriesContext(ctx context.Context, entries []runtimeReferenceEntry) (runtimeReferenceMap, error) {
	hasDomain := false
	for _, entry := range entries {
		hasDomain = hasDomain || entry.Key.containsDomain
	}
	if hasDomain && len(entries) > 1 {
		shape := runtimeRawSymbolicShape(entries[0].Key)
		for _, entry := range entries[1:] {
			if err := ctx.Err(); err != nil {
				return runtimeReferenceMap{}, err
			}
			if runtimeRawSymbolicShape(entry.Key) != shape {
				return runtimeReferenceMap{}, Diagnostic{Code: "QNT501", Message: "Mixed symbolic runtime collection representations require contextual equality"}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return runtimeReferenceMap{}, err
	}
	return runtimeReferenceMapFromEntries(entries)
}

func runtimeRawSymbolicShape(value RuntimeValue) string {
	var result strings.Builder
	var domainShape func(RuntimeDomain)
	var valueShape func(RuntimeValue)
	domainShape = func(domain RuntimeDomain) {
		result.WriteString("d")
		result.WriteString(strconv.Itoa(int(domain.kind)))
		result.WriteByte('(')
		switch domain.kind {
		case runtimeExplicitDomain:
			valueShape(domain.explicit)
		case runtimeProductDomain:
			for _, factor := range domain.factors {
				domainShape(factor)
			}
		case runtimePowersetDomain:
			domainShape(*domain.base)
		case runtimeMapSetDomain:
			domainShape(*domain.domain)
			domainShape(*domain.rangeSet)
		}
		result.WriteByte(')')
	}
	valueShape = func(value RuntimeValue) {
		if value.domain != nil {
			domainShape(*value.domain)
			return
		}
		result.WriteString(value.expression.Kind)
		result.WriteByte(':')
		result.WriteString(value.expression.Opcode)
		if value.containsDomain && value.payload != nil {
			result.WriteByte('(')
			if value.expression.Opcode == "Set" || value.expression.Opcode == "Map" {
				for _, entry := range value.payload.entries.Entries() {
					valueShape(entry.Key)
					valueShape(entry.Value)
				}
			} else {
				for _, child := range value.payload.children {
					valueShape(child)
				}
			}
			result.WriteByte(')')
		}
		result.WriteByte(';')
	}
	valueShape(value)
	return result.String()
}

// EvalSetToMapBuiltinContext preserves the intermediate fresh-array object
// hashes before normalizing the surviving keys and retaining map values.
func EvalSetToMapBuiltinContext(ctx context.Context, args []RuntimeValue, hashes *RuntimeObjectHashState) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if len(args) != 1 {
		return fail("setToMap requires 1 argument")
	}
	if !args[0].Valid() {
		return fail("Invalid runtime value")
	}
	if !runtimeContextSetLike(args[0]) {
		return fail("setToMap requires a Set value")
	}
	if hashes == nil {
		return fail("setToMap requires an object hash state")
	}
	set, err := runtimeToSetContext(ctx, args[0])
	if err != nil {
		return RuntimeValue{}, err
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
		pairs.Add(hashes, &runtimeObjectIdentity{}, runtimeReferenceEntry{Key: children[0], Value: children[1]})
	}
	raw := make([]runtimeReferenceEntry, 0, len(entries))
	for _, pair := range pairs.Entries() {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		raw = append(raw, pair.Value)
	}
	intermediate, err := runtimeRawMapFromEntriesContext(ctx, raw)
	if err != nil {
		return RuntimeValue{}, err
	}
	normalized := intermediate.Entries()
	for i, entry := range normalized {
		normalized[i].Key, err = runtimeKeyNormalFormContext(ctx, entry.Key)
		if err != nil {
			return RuntimeValue{}, err
		}
	}
	result, err := runtimeReferenceMapFromEntries(normalized)
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeWithEntriesContext(ctx, "Map", result)
}

// EvalAllListsBuiltinContext materializes its domain once, retains fresh-array
// hash history, and normalizes generated lists only at the reference boundary.
func EvalAllListsBuiltinContext(ctx context.Context, op string, args []RuntimeValue, hashes *RuntimeObjectHashState) (RuntimeValue, error) {
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
	if !runtimeContextSetLike(args[0]) {
		return fail("allListsUpTo requires a Set value")
	}
	if args[1].expression.Kind != "int" {
		return fail("allListsUpTo requires an integer length")
	}
	if hashes == nil {
		return fail("allListsUpTo requires an object hash state")
	}
	set, err := runtimeToSetContext(ctx, args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	entries, err := runtimeEntries(set)
	if err != nil {
		return RuntimeValue{}, err
	}
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
	results := make([]runtimeReferenceEntry, 0, len(lists.entries))
	for _, entry := range lists.Entries() {
		list, err := NewRuntimeCompositeContext(ctx, "List", entry.Value.values)
		if err != nil {
			return RuntimeValue{}, err
		}
		results = append(results, runtimeReferenceEntry{Key: list, Value: list})
	}
	mapped, err := runtimeRawMapFromEntriesContext(ctx, results)
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeContextSetFromEntries(ctx, mapped)
}
