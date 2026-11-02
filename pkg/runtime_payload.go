package quint

import (
	"fmt"
	"sort"

	"github.com/glycerine/insdict"
)

// A payload retains actual immutable children and collection node history.
// expression is a canonical display/identity cache; source is a copied view,
// never the storage used to perform runtime access or mutation.
type runtimePayload struct {
	children []RuntimeValue
	entries  runtimeReferenceMap
}

func runtimeChildren(value RuntimeValue) ([]RuntimeValue, error) {
	if value.payload == nil {
		return nil, Diagnostic{Code: "QNT501", Message: "Expected a concrete collection value"}
	}
	switch value.expression.Opcode {
	case "List", "Tup", "Rec", "variant":
		return append([]RuntimeValue{}, value.payload.children...), nil
	}
	return nil, Diagnostic{Code: "QNT501", Message: "Expected a sequence, record, or variant value"}
}
func runtimeMap(value RuntimeValue) (runtimeReferenceMap, error) {
	if value.payload != nil && (value.expression.Opcode == "Set" || value.expression.Opcode == "Map") {
		return value.payload.entries, nil
	}
	return runtimeReferenceMap{}, Diagnostic{Code: "QNT501", Message: "Expected a concrete set or map value"}
}
func runtimeEntries(value RuntimeValue) ([]runtimeReferenceEntry, error) {
	m, err := runtimeMap(value)
	if err != nil {
		return nil, err
	}
	return m.Entries(), nil
}

func runtimeAssemble(source Expr, payload *runtimePayload) (RuntimeValue, error) {
	normalized, err := NormalizeValue(source)
	if err != nil {
		return RuntimeValue{}, err
	}
	return RuntimeValue{expression: normalized, source: copyRuntimeExpression(source), identity: normalizedValueKey(normalized), allocation: &runtimeValueAllocation{}, payload: payload}, nil
}

// runtimeWithEntries wraps a retained tree without reconstructing its history.
func runtimeWithEntries(op string, m runtimeReferenceMap) (RuntimeValue, error) {
	source := Expr{Kind: "app", Opcode: op, Args: []Expr{}}
	for _, entry := range m.Entries() {
		switch op {
		case "Set":
			source.Args = append(source.Args, entry.Value.source)
		case "Map":
			source.Args = append(source.Args, Expr{Kind: "app", Opcode: "Tup", Args: []Expr{entry.Key.source, entry.Value.source}})
		default:
			return RuntimeValue{}, fmt.Errorf("invalid retained collection %s", op)
		}
	}
	if op != "Set" && op != "Map" {
		return RuntimeValue{}, fmt.Errorf("invalid retained collection %s", op)
	}
	return runtimeAssemble(source, &runtimePayload{entries: m})
}

// newRuntimeComposite implements runtime constructors on evaluated children.
// Child objects remain immutable and can be returned directly by accessors.
func newRuntimeComposite(op string, args []RuntimeValue) (RuntimeValue, error) {
	source := Expr{Kind: "app", Opcode: op, Args: make([]Expr, len(args))}
	for i, arg := range args {
		if !arg.Valid() {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
		}
		source.Args[i] = arg.source
	}
	switch op {
	case "List", "Tup":
		return runtimeAssemble(source, &runtimePayload{children: append([]RuntimeValue{}, args...)})
	case "Rec":
		if len(args)%2 != 0 {
			return RuntimeValue{}, fmt.Errorf("record requires field/value pairs")
		}
		fields := insdict.NewDict[string, [2]RuntimeValue]()
		for i := 0; i < len(args); i += 2 {
			if args[i].expression.Kind != "str" {
				return RuntimeValue{}, fmt.Errorf("record field must be a string")
			}
			fields.Set(args[i].expression.Str, [2]RuntimeValue{args[i], args[i+1]})
		}
		names := []string{}
		for name := range fields.All() {
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool { return utf16Less(names[i], names[j]) })
		children := make([]RuntimeValue, 0, len(names)*2)
		source.Args = []Expr{}
		for _, name := range names {
			pair := fields.Get(name)
			children = append(children, pair[0], pair[1])
			source.Args = append(source.Args, pair[0].source, pair[1].source)
		}
		return runtimeAssemble(source, &runtimePayload{children: children})
	case "variant":
		if len(args) != 2 || args[0].expression.Kind != "str" {
			return RuntimeValue{}, fmt.Errorf("variant requires a string label and value")
		}
		return runtimeAssemble(source, &runtimePayload{children: append([]RuntimeValue{}, args...)})
	case "Set":
		var m runtimeReferenceMap
		for _, value := range args {
			normalized, err := runtimeNormalForm(value)
			if err != nil {
				return RuntimeValue{}, err
			}
			m, err = m.Set(normalized, normalized)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return runtimeWithEntries(op, m)
	case "Map":
		entries := make([]runtimeReferenceEntry, 0, len(args))
		for _, pair := range args {
			if pair.expression.Kind != "app" || (pair.expression.Opcode != "Tup" && pair.expression.Opcode != "List") {
				return RuntimeValue{}, fmt.Errorf("map requires two-element tuple entries")
			}
			children, err := runtimeChildren(pair)
			if err != nil {
				return RuntimeValue{}, err
			}
			if len(children) != 2 {
				return RuntimeValue{}, fmt.Errorf("map requires two-element tuple entries")
			}
			key, err := runtimeNormalForm(children[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			entries = append(entries, runtimeReferenceEntry{Key: key, Value: children[1]})
		}
		m, err := runtimeReferenceMapFromEntries(entries)
		if err != nil {
			return RuntimeValue{}, err
		}
		return runtimeWithEntries(op, m)
	}
	return RuntimeValue{}, fmt.Errorf("Cannot convert %s to a runtime value", ExpressionString(source))
}

// runtimeNormalForm matches the reference's object-level normalization, which
// differs from canonical display sorting. In particular a map rebuilds its
// tree, a set keeps its tree, and a variant keeps its payload untouched.
func runtimeNormalForm(value RuntimeValue) (RuntimeValue, error) {
	if !value.Valid() {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	if value.expression.Kind != "app" {
		return value, nil
	}
	switch value.expression.Opcode {
	case "variant":
		return value, nil
	case "Set":
		m, err := runtimeMap(value)
		if err != nil {
			return RuntimeValue{}, err
		}
		return runtimeWithEntries("Set", m)
	case "Map":
		entries, err := runtimeEntries(value)
		if err != nil {
			return RuntimeValue{}, err
		}
		for i, entry := range entries {
			entries[i].Key, err = runtimeNormalForm(entry.Key)
			if err != nil {
				return RuntimeValue{}, err
			}
			entries[i].Value, err = runtimeNormalForm(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		m, err := runtimeReferenceMapFromEntries(entries)
		if err != nil {
			return RuntimeValue{}, err
		}
		return runtimeWithEntries("Map", m)
	case "List", "Tup", "Rec":
		children, err := runtimeChildren(value)
		if err != nil {
			return RuntimeValue{}, err
		}
		for i, child := range children {
			// Record names are primitive strings and normalize to themselves.
			children[i], err = runtimeNormalForm(child)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return newRuntimeComposite(value.expression.Opcode, children)
	}
	return value, nil
}
