package quint

import (
	"fmt"
	"sort"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// SerializeRuntimeValue implements the TypeScript runtime's toQuintEx operation
// with zero expression IDs. state accounts for the fresh JavaScript objects
// hashed while serializing sets. Reusing it preserves the reference's process
// history, which can change the order of otherwise tied display strings.
// Passing nil uses a fresh object-hash state. The returned expression is owned
// by the caller; serialization does not change value or its retained children.
//
// This explicit compatibility operation is separate from RuntimeValue.Expression,
// whose deterministic canonical representation defines the public Go boundary.
func SerializeRuntimeValue(value RuntimeValue, state *RuntimeObjectHashState) (Expr, error) {
	if state == nil {
		state = &RuntimeObjectHashState{}
	}
	serializer := runtimeSerialization{objects: state}
	return serializer.expression(value)
}

func runtimeSerialize(value RuntimeValue) (Expr, error) {
	return SerializeRuntimeValue(value, nil)
}

type runtimeSerialization struct {
	objects    *RuntimeObjectHashState
	comparison *collate.Collator
}

func (s *runtimeSerialization) expression(value RuntimeValue) (Expr, error) {
	if !value.Valid() {
		return Expr{}, fmt.Errorf("cannot serialize an invalid runtime value")
	}
	if value.closure != nil {
		return copyRuntimeExpression(value.expression), nil
	}
	switch value.expression.Kind {
	case "int":
		return Expr{Kind: "int", Int: value.expression.Int}, nil
	case "str":
		return Expr{Kind: "str", Str: value.expression.Str}, nil
	case "bool":
		return Expr{Kind: "bool", Bool: value.expression.Bool}, nil
	case "app":
	default:
		return Expr{}, fmt.Errorf("runtime serialization does not support %s", value.expression.Kind)
	}
	op := value.expression.Opcode
	switch op {
	case "List", "Tup", "Rec", "variant":
		children, err := runtimeChildren(value)
		if err != nil {
			return Expr{}, err
		}
		out := Expr{Kind: "app", Opcode: op, Args: make([]Expr, len(children))}
		for i, child := range children {
			out.Args[i], err = s.expression(child)
			if err != nil {
				return Expr{}, err
			}
		}
		return out, nil
	case "Set":
		entries, err := runtimeEntries(value)
		if err != nil {
			return Expr{}, err
		}
		values := make([]RuntimeValue, len(entries))
		for i, entry := range entries {
			values[i] = entry.Value
		}
		args, err := s.setElements(values)
		return Expr{Kind: "app", Opcode: "Set", Args: args}, err
	case "Map":
		entries, err := runtimeEntries(value)
		if err != nil {
			return Expr{}, err
		}
		// The reference first builds a transient Set of runtime tuple objects.
		// These have structural hashes and retain their actual child values.
		pairs := make([]runtimeReferenceEntry, len(entries))
		for i, entry := range entries {
			expression := Expr{Kind: "app", Opcode: "Tup", Args: []Expr{entry.Key.expression, entry.Value.expression}}
			containsClosure := entry.Key.containsClosure || entry.Value.containsClosure
			pair := RuntimeValue{
				expression:      expression,
				identity:        normalizedValueKey(expression),
				allocation:      &runtimeValueAllocation{},
				payload:         &runtimePayload{children: []RuntimeValue{entry.Key, entry.Value}, containsClosure: containsClosure},
				containsClosure: containsClosure,
			}
			pairs[i] = runtimeReferenceEntry{Key: pair, Value: pair}
		}
		set, err := runtimeReferenceMapFromEntries(pairs)
		if err != nil {
			return Expr{}, err
		}
		values := make([]RuntimeValue, 0, set.Len())
		for _, entry := range set.Entries() {
			values = append(values, entry.Value)
		}
		args, err := s.setElements(values)
		return Expr{Kind: "app", Opcode: "Map", Args: args}, err
	}
	return Expr{}, fmt.Errorf("runtime serialization does not support %s", op)
}

func (s *runtimeSerialization) setElements(values []RuntimeValue) ([]Expr, error) {
	// Both Set.map operations are significant. The first inserts fresh QuintEx
	// objects; the second inserts fresh copies with a cached printed string.
	// Their identity hashes are assigned lazily after the eighth entry, with
	// nested serialization completed before its parent object is inserted.
	var expressions runtimeObjectSet[Expr]
	for _, value := range values {
		expression, err := s.expression(value)
		if err != nil {
			return nil, err
		}
		expressions.Add(s.objects, &runtimeObjectIdentity{}, expression)
	}
	type printedExpression struct {
		expression Expr
		text       string
	}
	var printed runtimeObjectSet[printedExpression]
	for _, entry := range expressions.Entries() {
		expression := entry.Value
		printed.Add(s.objects, &runtimeObjectIdentity{}, printedExpression{expression, ExpressionString(expression)})
	}
	entries := printed.Entries()
	if len(entries) > 1 {
		if s.comparison == nil {
			s.comparison = collate.New(language.AmericanEnglish)
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return s.comparison.CompareString(entries[i].Value.text, entries[j].Value.text) < 0
		})
	}
	args := make([]Expr, len(entries))
	for i, entry := range entries {
		args[i] = entry.Value.expression
	}
	return args, nil
}
