package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

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
func DecodeITFTrace(data []byte) (ITFTrace, error) {
	var input struct {
		Vars, Params []string
		Loop         *int
		Metadata     json.RawMessage `json:"#meta"`
		States       []json.RawMessage
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return ITFTrace{}, err
	}
	if input.States == nil {
		return ITFTrace{}, fmt.Errorf("ITF trace requires states")
	}
	result := ITFTrace{Vars: input.Vars, Params: input.Params, Loop: input.Loop, Metadata: input.Metadata, States: []Expr{}, StateMetadata: []json.RawMessage{}, Raw: append(json.RawMessage(nil), data...)}
	ids := &IDGenerator{} // Upstream ofItf starts at zero and shares IDs across states.
	for i, state := range input.States {
		expression, err := DecodeITFValue(state, ids)
		if err != nil {
			return ITFTrace{}, fmt.Errorf("ITF state %d: %w", i, err)
		}
		var metadata struct {
			Value json.RawMessage `json:"#meta"`
		}
		if len(state) > 0 && bytes.TrimSpace(state)[0] == '{' {
			_ = json.Unmarshal(state, &metadata)
		}
		result.States = append(result.States, expression)
		result.StateMetadata = append(result.StateMetadata, metadata.Value)
	}
	return result, nil
}

// DecodeITFValue converts an ITF value to Quint IR without floating-point number
// conversion. Nil ids starts a new zero-based sequence; malformed values return
// errors rather than upstream's unchecked JavaScript exceptions.
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
		err := json.Unmarshal(data, &e.Str)
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
		e.Kind, e.Name = "name", *name
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
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields.Set(key.(string), value)
	}
	return fields, nil
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
