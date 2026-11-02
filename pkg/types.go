package quint

import (
	"encoding/json"
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
func (t *Type) UnmarshalJSON(b []byte) error {
	type plain Type
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	v := Type(p)
	if err := v.Validate(); err != nil {
		return err
	}
	*t = v
	return nil
}
func (r *Row) UnmarshalJSON(b []byte) error {
	type plain Row
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	v := Row(p)
	if err := v.Validate(); err != nil {
		return err
	}
	*r = v
	return nil
}
func UnitType() Type {
	return Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: []RowField{}, Other: &Row{Kind: "empty"}}}
}
func (t Type) IsUnit() bool {
	return t.Kind == "tup" && t.Fields != nil && t.Fields.Kind == "row" && len(t.Fields.Fields) == 0 && t.Fields.Other != nil && t.Fields.Other.Kind == "empty"
}
