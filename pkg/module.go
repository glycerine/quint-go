package quint

import (
	"encoding/json"
	"fmt"
)

type Module struct {
	ID           ID            `json:"id"`
	Name         string        `json:"name"`
	Declarations []Declaration `json:"declarations"`
	Doc          *string       `json:"doc,omitempty"`
}

// Declaration holds the fields of Quint's top-level tagged declaration union.
// Optional strings use pointers to preserve present-but-empty documentation.
type Declaration struct {
	// Runtime provenance survives in-memory flattening and ID refreshes. It is
	// deliberately absent from the transport IR and its custom JSON encoding.
	runtimeVariable *RuntimeVariableBinding

	Hidden       *bool           `json:"hidden,omitempty"`
	Namespaces   []string        `json:"namespaces,omitempty"`
	Shadowing    json.RawMessage `json:"shadowing,omitempty"`
	ImportedFrom json.RawMessage `json:"importedFrom,omitempty"`

	Depth            *int       `json:"depth,omitempty"`
	ID               ID         `json:"id"`
	Kind             string     `json:"kind"`
	Name             string     `json:"name,omitempty"`
	Qualifier        Qualifier  `json:"qualifier,omitempty"`
	Expr             *Expr      `json:"expr,omitempty"`
	Assumption       *Expr      `json:"assumption,omitempty"`
	TypeAnnotation   *Type      `json:"typeAnnotation,omitempty"`
	Type             *Type      `json:"type,omitempty"`
	Params           []string   `json:"params,omitempty"`
	Doc              *string    `json:"doc,omitempty"`
	ProtoName        string     `json:"protoName,omitempty"`
	DefName          *string    `json:"defName,omitempty"`
	QualifiedName    *string    `json:"qualifiedName,omitempty"`
	FromSource       *string    `json:"fromSource,omitempty"`
	Overrides        []Override `json:"overrides,omitempty"`
	IdentityOverride bool       `json:"identityOverride,omitempty"`
}

// Override is encoded as the two-element [parameter, expression] tuple upstream.
type Override struct {
	Parameter Parameter
	Expr      Expr
}

func (o Override) MarshalJSON() ([]byte, error) { return json.Marshal([2]any{o.Parameter, o.Expr}) }
func (o *Override) UnmarshalJSON(data []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	if len(pair) != 2 {
		return fmt.Errorf("instance override must have two elements")
	}
	var v Override
	if err := json.Unmarshal(pair[0], &v.Parameter); err != nil {
		return err
	}
	if string(pair[0]) == "null" || v.Parameter.Name == "" {
		return fmt.Errorf("invalid override parameter")
	}
	if err := json.Unmarshal(pair[1], &v.Expr); err != nil {
		return err
	}
	*o = v
	return nil
}
func (d Declaration) Validate() error {
	switch d.Kind {
	case "def":
		if d.Name == "" || !d.Qualifier.Valid() || d.Expr == nil {
			return fmt.Errorf("invalid operator declaration")
		}
	case "const", "var":
		if d.Name == "" || d.TypeAnnotation == nil {
			return fmt.Errorf("invalid %s declaration", d.Kind)
		}
	case "assume":
		if d.Name == "" || d.Assumption == nil {
			return fmt.Errorf("invalid assumption")
		}
	case "typedef":
		if d.Name == "" {
			return fmt.Errorf("type declaration missing name")
		}
	case "import", "export", "instance":
		if d.ProtoName == "" {
			return fmt.Errorf("%s missing module name", d.Kind)
		}
	default:
		return fmt.Errorf("unknown declaration kind %q", d.Kind)
	}
	return nil
}
func (d Declaration) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type plain Declaration
	data, err := json.Marshal(plain(d))
	if err != nil {
		return nil, err
	}
	if d.Kind != "instance" && !(d.Kind == "typedef" && d.Params != nil) {
		return data, nil
	}
	var obj map[string]json.RawMessage
	if err = json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if d.Kind == "typedef" {
		obj["params"], err = json.Marshal(d.Params)
		if err != nil {
			return nil, err
		}
		return json.Marshal(obj)
	}
	overrides := d.Overrides
	if overrides == nil {
		overrides = []Override{}
	}
	obj["overrides"], err = json.Marshal(overrides)
	if err != nil {
		return nil, err
	}
	obj["identityOverride"], err = json.Marshal(d.IdentityOverride)
	if err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}
func (d *Declaration) UnmarshalJSON(data []byte) error {
	if err := declarationSchema(data); err != nil {
		return err
	}
	type plain Declaration
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	v := Declaration(p)
	if err := v.Validate(); err != nil {
		return err
	}
	*d = v
	return nil
}
