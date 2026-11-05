package quint

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/glycerine/insdict"
)

// ITFTrace retains trace metadata and original JSON alongside display expressions.
// Expressions omit #meta and __-prefixed record fields, matching upstream ofItf.
// Raw retains those fields for lossless trace output and further interpretation.
type ITFTrace struct {
	Vars, Params  []string
	Loop          *int
	Metadata      json.RawMessage
	StateMetadata []json.RawMessage
	States        []Expr
	Raw           json.RawMessage
}

// DecodeITFTrace decodes all states with one zero-based expression ID sequence.
// It retains original bytes and trace/state metadata independently of the IR.
// Exposed strings and names must be Unicode scalar values; unsupported lone
// UTF-16 surrogates are errors instead of being replaced. Raw metadata is opaque.
func DecodeITFTrace(data []byte) (ITFTrace, error) {
	// Field names are matched exactly, as upstream's getFieldOpt does. A struct
	// target would let encoding/json match case-insensitively (e.g. "stAtes").
	var input map[string]json.RawMessage
	if err := json.Unmarshal(data, &input); err != nil {
		return ITFTrace{}, err
	}
	if input == nil {
		return ITFTrace{}, fmt.Errorf("ITF trace must be an object")
	}
	states, ok := input["states"]
	if !ok || itfIsNull(states) {
		return ITFTrace{}, fmt.Errorf("ITF trace requires states")
	}
	var stateList []json.RawMessage
	if err := json.Unmarshal(states, &stateList); err != nil {
		return ITFTrace{}, fmt.Errorf("ITF states: %w", err)
	}
	vars, err := itfNameList("vars", input["vars"])
	if err != nil {
		return ITFTrace{}, err
	}
	params, err := itfNameList("params", input["params"])
	if err != nil {
		return ITFTrace{}, err
	}
	var loop *int
	if raw, ok := input["loop"]; ok && !itfIsNull(raw) {
		var index int
		if err := json.Unmarshal(raw, &index); err != nil {
			return ITFTrace{}, fmt.Errorf("ITF loop: %w", err)
		}
		loop = &index
	}
	result := ITFTrace{Vars: vars, Params: params, Loop: loop, Metadata: input["#meta"], States: []Expr{}, StateMetadata: []json.RawMessage{}, Raw: append(json.RawMessage(nil), data...)}
	ids := &IDGenerator{} // Upstream ofItf starts at zero and shares IDs across states.
	for i, state := range stateList {
		// Upstream reads each state with allFieldsOpt, so only objects are states.
		// Accepting a bare value here would produce a trace the encoder rejects.
		if !itfIsObject(state) {
			return ITFTrace{}, fmt.Errorf("ITF state %d: State must be an object.", i)
		}
		expression, err := DecodeITFValue(state, ids)
		if err != nil {
			return ITFTrace{}, fmt.Errorf("ITF state %d: %w", i, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(state, &fields); err != nil {
			return ITFTrace{}, fmt.Errorf("ITF state %d: %w", i, err)
		}
		result.States = append(result.States, expression)
		result.StateMetadata = append(result.StateMetadata, fields["#meta"])
	}
	return result, nil
}

// itfIsNull reports whether raw is the JSON literal null.
func itfIsNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// itfIsObject reports whether raw is a JSON object, ignoring surrounding space.
func itfIsObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// itfNameList decodes an optional vars or params array. Absent and null are
// both empty, matching upstream's optional handling of these fields.
func itfNameList(field string, raw json.RawMessage) ([]string, error) {
	if raw == nil || itfIsNull(raw) {
		return nil, nil
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("ITF %s: %w", field, err)
	}
	return itfUnicodeNames(field, values)
}

// DecodeITFValue converts an ITF value to Quint IR without floating-point number
// conversion. Nil ids starts a new zero-based sequence; malformed values return
// errors rather than upstream's unchecked JavaScript exceptions.
// Lone UTF-16 surrogates cannot be represented by the Go string boundary and
// are rejected; valid surrogate pairs are decoded to their UTF-8 scalar value.
func DecodeITFValue(data []byte, ids *IDGenerator) (Expr, error) {
	if ids == nil {
		ids = &IDGenerator{}
	}
	return decodeITFValue(data, ids.Next)
}

func decodeITFValue(data []byte, nextID func() ID) (Expr, error) {
	data = bytes.TrimSpace(data)
	if !json.Valid(data) {
		return Expr{}, fmt.Errorf("invalid ITF JSON")
	}
	e := Expr{ID: nextID()}
	decode := func(raw json.RawMessage) (Expr, error) { return decodeITFValue(raw, nextID) }
	array := func(raw json.RawMessage) ([]Expr, error) {
		var values []json.RawMessage
		if len(raw) == 0 || bytes.TrimSpace(raw)[0] != '[' {
			return nil, fmt.Errorf("ITF collection requires an array")
		}
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
		out := []Expr{}
		for _, value := range values {
			x, err := decode(value)
			if err != nil {
				return nil, err
			}
			out = append(out, x)
		}
		return out, nil
	}
	switch data[0] {
	case 't', 'f':
		e.Kind = "bool"
		err := json.Unmarshal(data, &e.Bool)
		return e, err
	case '"':
		e.Kind = "str"
		text, err := itfUnicodeString(data)
		e.Str = text
		return e, err
	case '[':
		e.Kind, e.Opcode = "app", "List"
		args, err := array(data)
		e.Args = args
		return e, err
	case 'n':
		return Expr{}, fmt.Errorf("null is not an ITF value")
	case '{':
	default:
		value, ok := new(big.Rat).SetString(string(data))
		if !ok || !value.IsInt() {
			return Expr{}, fmt.Errorf("ITF number is not an integer: %s", data)
		}
		e.Kind, e.Int = "int", IntegerFromBig(value.Num())
		return e, nil
	}
	fields, err := itfObject(data)
	if err != nil {
		return Expr{}, err
	}
	if value, ok := fields.Get2("#bigint"); ok {
		var text string
		if err = json.Unmarshal(value, &text); err != nil {
			return Expr{}, fmt.Errorf("ITF #bigint requires a string")
		}
		e.Kind = "int"
		e.Int, err = ParseInteger(text)
		return e, err
	}
	for _, tag := range []struct{ key, opcode string }{{"#tup", "Tup"}, {"#set", "Set"}} {
		if value, ok := fields.Get2(tag.key); ok {
			e.Kind, e.Opcode = "app", tag.opcode
			e.Args, err = array(value)
			return e, err
		}
	}
	if value, ok := fields.Get2("#unserializable"); ok {
		var name *string
		if err = json.Unmarshal(value, &name); err != nil || name == nil {
			return Expr{}, fmt.Errorf("ITF #unserializable requires a string")
		}
		text, err := itfUnicodeString(value)
		if err != nil {
			return Expr{}, err
		}
		e.Kind, e.Name = "name", text
		return e, nil
	}
	if value, ok := fields.Get2("#map"); ok {
		var pairs []json.RawMessage
		if len(value) == 0 || bytes.TrimSpace(value)[0] != '[' {
			return Expr{}, fmt.Errorf("ITF #map requires an array")
		}
		if err = json.Unmarshal(value, &pairs); err != nil {
			return Expr{}, err
		}
		e.Kind, e.Opcode, e.Args = "app", "Map", []Expr{}
		for _, pair := range pairs {
			var entries []json.RawMessage
			if err = json.Unmarshal(pair, &entries); err != nil || len(entries) != 2 {
				return Expr{}, fmt.Errorf("ITF map entry requires two values")
			}
			key, err := decode(entries[0])
			if err != nil {
				return Expr{}, err
			}
			value, err := decode(entries[1])
			if err != nil {
				return Expr{}, err
			}
			e.Args = append(e.Args, Expr{ID: nextID(), Kind: "app", Opcode: "Tup", Args: []Expr{key, value}})
		}
		return e, nil
	}
	if label, ok := fields.Get2("tag"); ok && fields.Len() <= 2 {
		payload, hasPayload := fields.Get2("value")
		if fields.Len() == 1 || hasPayload {
			label, err := decode(label)
			if err != nil {
				return Expr{}, err
			}
			e.Kind = "app"
			if hasPayload {
				value, err := decode(payload)
				if err != nil {
					return Expr{}, err
				}
				e.Opcode, e.Args = "variant", []Expr{label, value}
				return e, nil
			}
			if label.Kind == "str" && label.Str == "UNIT" {
				e.Opcode, e.Args = "Tup", []Expr{}
				return e, nil
			}
			e.Opcode, e.Args = "Rec", []Expr{{ID: nextID(), Kind: "str", Str: "tag"}, label}
			return e, nil
		}
	}
	e.Kind, e.Opcode, e.Args = "app", "Rec", []Expr{}
	for _, key := range itfObjectKeys(fields) {
		if key == "#meta" || strings.HasPrefix(key, "__") {
			continue
		}
		name := Expr{ID: nextID(), Kind: "str", Str: key}
		value, err := decode(fields.Get(key))
		if err != nil {
			return Expr{}, err
		}
		e.Args = append(e.Args, name, value)
	}
	return e, nil
}

func itfObject(data []byte) (*insdict.Dict[string, json.RawMessage], error) {
	fields := insdict.NewDict[string, json.RawMessage]()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	for decoder.More() {
		start := decoder.InputOffset()
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		// encoding/json replaces lone UTF-16 surrogates with U+FFFD. Refuse
		// those keys before distinct source properties can silently collapse.
		rawKey := bytes.TrimLeft(data[start:decoder.InputOffset()], ", \t\r\n")
		if bytes.Contains(rawKey, []byte(`\u`)) || !utf8.Valid(rawKey) {
			if _, err := itfUnicodeString(rawKey); err != nil {
				return nil, fmt.Errorf("ITF object field: %w", err)
			}
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields.Set(key.(string), value)
	}
	return fields, nil
}

// ITF's JavaScript producer can hold lone UTF-16 code units. Go strings expose
// Unicode scalar values here; reject unsupported strings instead of replacing
// distinct values with U+FFFD. Valid surrogate pairs remain ordinary UTF-8.
func itfUnicodeString(data []byte) (string, error) {
	text, err := jsontext.AppendUnquote(nil, bytes.TrimSpace(data))
	if err != nil {
		return "", fmt.Errorf("ITF string is not valid Unicode: %w", err)
	}
	return string(text), nil
}

func itfUnicodeNames(field string, raw []json.RawMessage) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	names := make([]string, len(raw))
	for i, value := range raw {
		name, err := itfUnicodeString(value)
		if err != nil {
			return nil, fmt.Errorf("ITF %s name %d: %w", field, i, err)
		}
		names[i] = name
	}
	return names, nil
}

// JavaScript enumerates canonical uint32 index keys before other object keys.
func itfObjectKeys(fields *insdict.Dict[string, json.RawMessage]) []string {
	indices, others := []uint64{}, []string{}
	for key := range fields.All() {
		index, err := strconv.ParseUint(key, 10, 32)
		if err == nil && index < 4294967295 && strconv.FormatUint(index, 10) == key {
			indices = append(indices, index)
		} else {
			others = append(others, key)
		}
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	keys := []string{}
	for _, index := range indices {
		keys = append(keys, strconv.FormatUint(index, 10))
	}
	return append(keys, others...)
}
