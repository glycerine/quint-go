package quint

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
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
	runtimeSource *runtimeSourceIdentity

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

// expressionJSON retains raw values until the tag determines their types. A
// fixed field layout avoids allocating a map and a second dispatch map at every
// node. The fallback preserves strict rejection of unknown and mis-cased names.
type expressionJSON struct {
	ID        json.RawMessage `json:"id"`
	Kind      json.RawMessage `json:"kind"`
	Name      json.RawMessage `json:"name"`
	Value     json.RawMessage `json:"value"`
	Opcode    json.RawMessage `json:"opcode"`
	Args      json.RawMessage `json:"args"`
	Params    json.RawMessage `json:"params"`
	Qualifier json.RawMessage `json:"qualifier"`
	Expr      json.RawMessage `json:"expr"`
	Opdef     json.RawMessage `json:"opdef"`
	Extra     jsontext.Value  `json:",embed"`
}

var expressionJSONOptions = jsonv2.JoinOptions(json.DefaultOptionsV1(), jsonv2.MatchCaseInsensitiveNames(false))

func (e *Expr) UnmarshalJSON(data []byte) error {
	var wire expressionJSON
	if err := jsonv2.Unmarshal(data, &wire, expressionJSONOptions); err != nil {
		return err
	}
	var v Expr
	read := func(key string, value *json.RawMessage, target any) error {
		data := *value
		if len(data) == 0 || string(data) == "null" {
			return fmt.Errorf("expression missing %s", key)
		}
		if err := json.Unmarshal(data, target); err != nil {
			return err
		}
		*value = nil
		return nil
	}
	if err := read("id", &wire.ID, &v.ID); err != nil {
		return err
	}
	if err := read("kind", &wire.Kind, &v.Kind); err != nil {
		return err
	}
	var err error
	switch v.Kind {
	case "name":
		err = read("name", &wire.Name, &v.Name)
	case "bool":
		err = read("value", &wire.Value, &v.Bool)
	case "int":
		err = read("value", &wire.Value, &v.Int)
	case "str":
		err = read("value", &wire.Value, &v.Str)
	case "app":
		err = read("opcode", &wire.Opcode, &v.Opcode)
		if err == nil {
			err = read("args", &wire.Args, &v.Args)
		}
	case "lambda":
		err = read("params", &wire.Params, &v.Params)
		if err == nil {
			err = read("qualifier", &wire.Qualifier, &v.Qualifier)
		}
		if err == nil {
			err = read("expr", &wire.Expr, &v.Body)
		}
	case "let":
		err = read("opdef", &wire.Opdef, &v.Opdef)
		if err == nil {
			err = read("expr", &wire.Expr, &v.Body)
		}
	default:
		return fmt.Errorf("unknown expression kind %q", v.Kind)
	}
	if err != nil {
		return err
	}
	if len(wire.Name)+len(wire.Value)+len(wire.Opcode)+len(wire.Args)+len(wire.Params)+len(wire.Qualifier)+len(wire.Expr)+len(wire.Opdef)+len(wire.Extra) != 0 {
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
