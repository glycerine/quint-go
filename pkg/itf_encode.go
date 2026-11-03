package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/glycerine/insdict"
)

const (
	ITFActionTaken = "mbt::actionTaken"
	ITFNondetPicks = "mbt::nondetPicks"
)

// EncodeITFValue converts a concrete Quint value expression to ITF JSON. It
// preserves collection order and writes all integers as decimal #bigint strings.
// Symbolic expressions and malformed collection constructors return errors.
func EncodeITFValue(expression Expr) (json.RawMessage, error) {
	e := expression
	marshal := func(value any) (json.RawMessage, error) { return json.Marshal(value) }
	object := func(key string, value any) (json.RawMessage, error) {
		encoded, err := marshal(value)
		if err != nil {
			return nil, err
		}
		fields := insdict.NewDict[string, json.RawMessage]()
		fields.Set(key, encoded)
		return marshal(itfEncodedObject{fields})
	}
	switch e.Kind {
	case "int":
		return object("#bigint", e.Int.String())
	case "str":
		if !utf8.ValidString(e.Str) {
			return nil, fmt.Errorf("ITF string is not valid UTF-8")
		}
		return marshal(e.Str)
	case "bool":
		return marshal(e.Bool)
	case "app":
	default:
		return nil, fmt.Errorf("Unexpected expression kind: %s", e.Kind)
	}
	if e.Opcode != "List" && e.Opcode != "Set" && e.Opcode != "Tup" && e.Opcode != "Rec" && e.Opcode != "Map" && e.Opcode != "variant" {
		return nil, fmt.Errorf("Unexpected operator type: %s", e.Opcode)
	}
	if e.Opcode == "Rec" && len(e.Args)%2 != 0 {
		return nil, fmt.Errorf("record: expected an even number of arguments, found:%d", len(e.Args))
	}
	if e.Opcode == "variant" && len(e.Args) != 2 {
		return nil, fmt.Errorf("variant requires a label and value")
	}
	args := []json.RawMessage{}
	for _, arg := range e.Args {
		value, err := EncodeITFValue(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	switch e.Opcode {
	case "List":
		return marshal(args)
	case "Set":
		return object("#set", args)
	case "Tup":
		return object("#tup", args)
	case "Rec":
		fields := insdict.NewDict[string, json.RawMessage]()
		for i := 0; i < len(args); i += 2 {
			if e.Args[i].Kind != "str" {
				return nil, fmt.Errorf("Invalid record field: %s", ExpressionString(e.Args[i]))
			}
			fields.Set(e.Args[i].Str, args[i+1])
		}
		return marshal(itfEncodedObject{fields})
	case "Map":
		pairs := [][2]json.RawMessage{}
		for i, arg := range e.Args {
			if arg.Kind != "app" || arg.Opcode != "Tup" || len(arg.Args) != 2 {
				return nil, fmt.Errorf("Invalid value in quint Map at entry %d", i)
			}
			var tuple struct {
				Values []json.RawMessage `json:"#tup"`
			}
			if err := json.Unmarshal(args[i], &tuple); err != nil {
				return nil, err
			}
			pairs = append(pairs, [2]json.RawMessage{tuple.Values[0], tuple.Values[1]})
		}
		return object("#map", pairs)
	case "variant":
		fields := insdict.NewDict[string, json.RawMessage]()
		fields.Set("tag", args[0])
		fields.Set("value", args[1])
		return marshal(itfEncodedObject{fields})
	}
	return nil, fmt.Errorf("Unexpected operator type: %s", e.Opcode)
}

// EncodeITFTrace matches upstream toItf: add state index metadata and optionally
// append model-based-testing variable names. It does not add a trace header or
// reorder/normalize values. Existing state #meta fields override index metadata.
func EncodeITFTrace(vars []string, states []Expr, mbtMetadata bool) (json.RawMessage, error) {
	for i, name := range vars {
		if !utf8.ValidString(name) {
			return nil, fmt.Errorf("ITF variable name %d is not valid UTF-8", i)
		}
	}
	names := append([]string{}, vars...)
	if mbtMetadata {
		names = append(names, ITFActionTaken, ITFNondetPicks)
	}
	encodedStates := []json.RawMessage{}
	for index, state := range states {
		value, err := EncodeITFValue(state)
		if err != nil {
			return nil, err
		}
		fields := insdict.NewDict[string, json.RawMessage]()
		metadata, _ := json.Marshal(struct {
			Index int `json:"index"`
		}{index})
		fields.Set("#meta", metadata)
		switch value[0] {
		case '{':
			object, err := itfObject(value)
			if err != nil {
				return nil, err
			}
			for name, field := range object.All() {
				fields.Set(name, field)
			}
		case '[':
			var values []json.RawMessage
			if err = json.Unmarshal(value, &values); err != nil {
				return nil, err
			}
			for i, item := range values {
				fields.Set(strconv.Itoa(i), item)
			}
		default:
			detail := string(value)
			if state.Kind == "str" {
				detail = state.Str
			}
			return nil, fmt.Errorf("Expected a valid ITF state, but found %s", detail)
		}
		encoded, err := json.Marshal(itfEncodedObject{fields})
		if err != nil {
			return nil, err
		}
		encodedStates = append(encodedStates, encoded)
	}
	return json.Marshal(struct {
		Vars   []string          `json:"vars"`
		States []json.RawMessage `json:"states"`
	}{names, encodedStates})
}

type itfEncodedObject struct {
	fields *insdict.Dict[string, json.RawMessage]
}

func (o itfEncodedObject) MarshalJSON() ([]byte, error) {
	var output bytes.Buffer
	output.WriteByte('{')
	for i, key := range itfObjectKeys(o.fields) {
		if i > 0 {
			output.WriteByte(',')
		}
		name, _ := json.Marshal(key)
		output.Write(name)
		output.WriteByte(':')
		output.Write(o.fields.Get(key))
	}
	output.WriteByte('}')
	return output.Bytes(), nil
}
