package quint

import (
	"encoding/json"
	"fmt"
)

type Qualifier string

const (
	PureVal  Qualifier = "pureval"
	PureDef  Qualifier = "puredef"
	Val      Qualifier = "val"
	Def      Qualifier = "def"
	Nondet   Qualifier = "nondet"
	Action   Qualifier = "action"
	Run      Qualifier = "run"
	Temporal Qualifier = "temporal"
)

func (q Qualifier) Valid() bool {
	switch q {
	case PureVal, PureDef, Val, Def, Nondet, Action, Run, Temporal:
		return true
	}
	return false
}

type Parameter struct {
	Shadowing      json.RawMessage `json:"shadowing,omitempty"`
	Depth          *int            `json:"depth,omitempty"`
	Kind           string          `json:"kind,omitempty"`
	ID             ID              `json:"id"`
	Name           string          `json:"name"`
	TypeAnnotation *Type           `json:"typeAnnotation,omitempty"`
}

// Expr represents the seven Quint expression variants. Literal values have
// dedicated Go fields, avoiding interface{} and floating-point JSON conversion.
type Expr struct {
	ID        ID
	Kind      string
	Name      string
	Bool      bool
	Int       Integer
	Str       string
	Opcode    string
	Args      []Expr
	Params    []Parameter
	Qualifier Qualifier
	Body      *Expr
	Opdef     *OperatorDef
}
type OperatorDef struct {
	Hidden       *bool           `json:"hidden,omitempty"`
	Namespaces   []string        `json:"namespaces,omitempty"`
	Shadowing    json.RawMessage `json:"shadowing,omitempty"`
	ImportedFrom json.RawMessage `json:"importedFrom,omitempty"`

	Depth          *int      `json:"depth,omitempty"`
	ID             ID        `json:"id"`
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	Qualifier      Qualifier `json:"qualifier"`
	Expr           *Expr     `json:"expr"`
	TypeAnnotation *Type     `json:"typeAnnotation,omitempty"`
	Doc            *string   `json:"doc,omitempty"`
}

func (e Expr) MarshalJSON() ([]byte, error) {
	obj := map[string]any{"id": e.ID, "kind": e.Kind}
	switch e.Kind {
	case "name":
		obj["name"] = e.Name
	case "bool":
		obj["value"] = e.Bool
	case "int":
		obj["value"] = e.Int
	case "str":
		obj["value"] = e.Str
	case "app":
		obj["opcode"] = e.Opcode
		args := e.Args
		if args == nil {
			args = []Expr{}
		}
		obj["args"] = args
	case "lambda":
		if e.Body == nil || !e.Qualifier.Valid() {
			return nil, fmt.Errorf("invalid lambda")
		}
		params := e.Params
		if params == nil {
			params = []Parameter{}
		}
		obj["params"] = params
		obj["qualifier"] = e.Qualifier
		obj["expr"] = e.Body
	case "let":
		if e.Body == nil || e.Opdef == nil {
			return nil, fmt.Errorf("invalid let")
		}
		obj["opdef"] = e.Opdef
		obj["expr"] = e.Body
	default:
		return nil, fmt.Errorf("unknown expression kind %q", e.Kind)
	}
	return json.Marshal(obj)
}
func (e *Expr) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	var v Expr
	read := func(key string, target any) error {
		b, ok := obj[key]
		if !ok || string(b) == "null" {
			return fmt.Errorf("expression missing %s", key)
		}
		delete(obj, key)
		return json.Unmarshal(b, target)
	}
	if err := read("id", &v.ID); err != nil {
		return err
	}
	if err := read("kind", &v.Kind); err != nil {
		return err
	}
	fields := map[string]any{}
	switch v.Kind {
	case "name":
		fields["name"] = &v.Name
	case "bool":
		fields["value"] = &v.Bool
	case "int":
		fields["value"] = &v.Int
	case "str":
		fields["value"] = &v.Str
	case "app":
		fields["opcode"] = &v.Opcode
		fields["args"] = &v.Args
	case "lambda":
		fields["params"] = &v.Params
		fields["qualifier"] = &v.Qualifier
		fields["expr"] = &v.Body
	case "let":
		fields["opdef"] = &v.Opdef
		fields["expr"] = &v.Body
	default:
		return fmt.Errorf("unknown expression kind %q", v.Kind)
	}
	for key, target := range fields {
		if err := read(key, target); err != nil {
			return err
		}
	}
	if len(obj) != 0 {
		return fmt.Errorf("unexpected fields in %s expression", v.Kind)
	}
	if v.Kind == "lambda" && !v.Qualifier.Valid() {
		return fmt.Errorf("unknown lambda qualifier %q", v.Qualifier)
	}
	if v.Kind == "let" && (v.Opdef.Kind != "def" || v.Opdef.Expr == nil || !v.Opdef.Qualifier.Valid()) {
		return fmt.Errorf("invalid let operator definition")
	}
	*e = v
	return nil
}
