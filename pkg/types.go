package quint

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
)

// Type is Quint's tagged type tree. Fields used by each kind follow quintTypes.ts.
// Trees are owned by the caller; transformations must copy nodes they modify.
type Type struct {
	ID     *ID    `json:"id,omitempty"`
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Elem   *Type  `json:"elem,omitempty"`
	Arg    *Type  `json:"arg,omitempty"`
	Res    *Type  `json:"res,omitempty"`
	Args   []Type `json:"args,omitempty"`
	Fields *Row   `json:"fields,omitempty"`
	Ctor   *Type  `json:"ctor,omitempty"`
}
type Row struct {
	Kind   string     `json:"kind"`
	Name   string     `json:"name,omitempty"`
	Fields []RowField `json:"fields,omitempty"`
	Other  *Row       `json:"other,omitempty"`
}
type RowField struct {
	Name string `json:"fieldName"`
	Type Type   `json:"fieldType"`
}
type TypeScheme struct {
	Type          Type     `json:"type"`
	TypeVariables []string `json:"typeVariables"`
	RowVariables  []string `json:"rowVariables"`
}

func (t Type) Validate() error {
	need := func(v *Type) error {
		if v == nil {
			return fmt.Errorf("%s type missing child", t.Kind)
		}
		return v.Validate()
	}
	switch t.Kind {
	case "bool", "int", "str":
	case "const", "var":
		if t.Name == "" {
			return fmt.Errorf("%s type missing name", t.Kind)
		}
	case "set", "list":
		return need(t.Elem)
	case "fun":
		if err := need(t.Arg); err != nil {
			return err
		}
		return need(t.Res)
	case "oper", "app":
		for _, a := range t.Args {
			if err := a.Validate(); err != nil {
				return err
			}
		}
		if t.Kind == "oper" {
			return need(t.Res)
		}
		if t.Ctor == nil || t.Ctor.Kind != "const" {
			return fmt.Errorf("type application requires constant constructor")
		}
		return t.Ctor.Validate()
	case "tup", "rec", "sum":
		if t.Fields == nil {
			return fmt.Errorf("%s type missing row", t.Kind)
		}
		if t.Kind == "sum" && t.Fields.Kind != "row" {
			return fmt.Errorf("sum requires concrete row")
		}
		return t.Fields.Validate()
	default:
		return fmt.Errorf("unknown type kind %q", t.Kind)
	}
	return nil
}
func (r Row) Validate() error {
	switch r.Kind {
	case "empty":
		return nil
	case "var":
		if r.Name == "" {
			return fmt.Errorf("row variable missing name")
		}
		return nil
	case "row":
		for _, f := range r.Fields {
			if err := f.Type.Validate(); err != nil {
				return err
			}
		}
		if r.Other == nil {
			return fmt.Errorf("row missing tail")
		}
		return r.Other.Validate()
	default:
		return fmt.Errorf("unknown row kind %q", r.Kind)
	}
}

// MarshalJSON retains required empty arrays (nullary operators and unit tuples).
func (t Type) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	type plain Type
	b, err := json.Marshal(plain(t))
	if err != nil {
		return nil, err
	}
	if t.Kind != "oper" && t.Kind != "app" {
		return b, nil
	}
	var obj map[string]json.RawMessage
	if err = json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	args := t.Args
	if args == nil {
		args = []Type{}
	}
	obj["args"], err = json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}
func (r Row) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type plain Row
	if r.Kind != "row" {
		return json.Marshal(plain(r))
	}
	fields := r.Fields
	if fields == nil {
		fields = []RowField{}
	}
	return json.Marshal(struct {
		Kind   string     `json:"kind"`
		Fields []RowField `json:"fields"`
		Other  *Row       `json:"other"`
	}{r.Kind, fields, r.Other})
}

// Raw fields let the tag select its schema before decoding children. In
// particular, a known field belonging to another variant must be rejected even
// when it contains null, false, or an empty array.
type typeJSON struct {
	ID     json.RawMessage `json:"id"`
	Kind   string          `json:"kind"`
	Name   json.RawMessage `json:"name"`
	Elem   json.RawMessage `json:"elem"`
	Arg    json.RawMessage `json:"arg"`
	Res    json.RawMessage `json:"res"`
	Args   json.RawMessage `json:"args"`
	Fields json.RawMessage `json:"fields"`
	Ctor   json.RawMessage `json:"ctor"`
	Extra  jsontext.Value  `json:",embed"`
}

type rowJSON struct {
	Kind   string          `json:"kind"`
	Name   json.RawMessage `json:"name"`
	Fields json.RawMessage `json:"fields"`
	Other  json.RawMessage `json:"other"`
	Extra  jsontext.Value  `json:",embed"`
}

type rowFieldJSON struct {
	Name  json.RawMessage `json:"fieldName"`
	Type  json.RawMessage `json:"fieldType"`
	Extra jsontext.Value  `json:",embed"`
}

var typeJSONOptions = jsonv2.JoinOptions(json.DefaultOptionsV1(), jsonv2.MatchCaseInsensitiveNames(false))

// Absence and null retain their existing typed-decoding behavior. Required
// semantic children/names are checked by Validate after the selected fields
// have been consumed; optional IDs and nil arrays remain accepted.
func readTypeJSONField(raw *json.RawMessage, target any) error {
	if len(*raw) > 0 {
		if err := json.Unmarshal(*raw, target); err != nil {
			return err
		}
		*raw = nil
	}
	return nil
}

func (t *Type) UnmarshalJSON(b []byte) error {
	var wire typeJSON
	if err := jsonv2.Unmarshal(b, &wire, typeJSONOptions); err != nil {
		return err
	}
	v := Type{Kind: wire.Kind}
	if err := readTypeJSONField(&wire.ID, &v.ID); err != nil {
		return err
	}
	var err error
	switch v.Kind {
	case "bool", "int", "str":
	case "const", "var":
		err = readTypeJSONField(&wire.Name, &v.Name)
	case "set", "list":
		err = readTypeJSONField(&wire.Elem, &v.Elem)
	case "fun":
		err = readTypeJSONField(&wire.Arg, &v.Arg)
		if err == nil {
			err = readTypeJSONField(&wire.Res, &v.Res)
		}
	case "oper":
		err = readTypeJSONField(&wire.Args, &v.Args)
		if err == nil {
			err = readTypeJSONField(&wire.Res, &v.Res)
		}
	case "app":
		err = readTypeJSONField(&wire.Args, &v.Args)
		if err == nil {
			err = readTypeJSONField(&wire.Ctor, &v.Ctor)
		}
	case "tup", "rec", "sum":
		err = readTypeJSONField(&wire.Fields, &v.Fields)
	default:
		return fmt.Errorf("unknown type kind %q", v.Kind)
	}
	if err != nil {
		return err
	}
	if len(wire.Name)+len(wire.Elem)+len(wire.Arg)+len(wire.Res)+len(wire.Args)+len(wire.Fields)+len(wire.Ctor)+len(wire.Extra) != 0 {
		return fmt.Errorf("unexpected fields in %s type", v.Kind)
	}
	if err := v.Validate(); err != nil {
		return err
	}
	*t = v
	return nil
}
func (r *Row) UnmarshalJSON(b []byte) error {
	var wire rowJSON
	if err := jsonv2.Unmarshal(b, &wire, typeJSONOptions); err != nil {
		return err
	}
	v := Row{Kind: wire.Kind}
	var err error
	switch v.Kind {
	case "empty":
	case "var":
		err = readTypeJSONField(&wire.Name, &v.Name)
	case "row":
		err = readTypeJSONField(&wire.Fields, &v.Fields)
		if err == nil {
			err = readTypeJSONField(&wire.Other, &v.Other)
		}
	default:
		return fmt.Errorf("unknown row kind %q", v.Kind)
	}
	if err != nil {
		return err
	}
	if len(wire.Name)+len(wire.Fields)+len(wire.Other)+len(wire.Extra) != 0 {
		return fmt.Errorf("unexpected fields in %s row", v.Kind)
	}
	if err := v.Validate(); err != nil {
		return err
	}
	*r = v
	return nil
}

func (f *RowField) UnmarshalJSON(b []byte) error {
	var wire rowFieldJSON
	if err := jsonv2.Unmarshal(b, &wire, typeJSONOptions); err != nil {
		return err
	}
	if len(wire.Extra) != 0 {
		return fmt.Errorf("unexpected fields in row field")
	}
	// RowField previously used the standard struct decoder, so absent fields
	// (and null string fields) retain their receiver values on a successful
	// partial decode. Stage changes locally to preserve it on any failure.
	v := *f
	if err := readTypeJSONField(&wire.Name, &v.Name); err != nil {
		return err
	}
	if err := readTypeJSONField(&wire.Type, &v.Type); err != nil {
		return err
	}
	*f = v
	return nil
}
func UnitType() Type {
	return Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: []RowField{}, Other: &Row{Kind: "empty"}}}
}
func (t Type) IsUnit() bool {
	return t.Kind == "tup" && t.Fields != nil && t.Fields.Kind == "row" && len(t.Fields.Fields) == 0 && t.Fields.Other != nil && t.Fields.Other.Kind == "empty"
}
