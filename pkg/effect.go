package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Effect struct {
	Kind       string            `json:"kind"`
	Components []EffectComponent `json:"components,omitempty"`
	Params     []Effect          `json:"params,omitempty"`
	Result     *Effect           `json:"result,omitempty"`
	Name       string            `json:"name,omitempty"`
}
type EffectComponent struct {
	Kind   string `json:"kind"`
	Entity Entity `json:"entity"`
}
type Entity struct {
	Kind           string          `json:"kind"`
	StateVariables []StateVariable `json:"stateVariables,omitempty"`
	Name           string          `json:"name,omitempty"`
	Reference      *ID             `json:"reference,omitempty"`
	Entities       []Entity        `json:"entities,omitempty"`
}

// StateVariable is a state-variable occurrence. Use NewStateVariable when
// programmatically constructing an occurrence that will be shared or copied.
type StateVariable struct {
	identity  *int   // Preserves occurrence identity across internal copies; never serialized.
	Name      string `json:"name"`
	Reference ID     `json:"reference"`
}

// NewStateVariable creates a distinct state-variable occurrence. Copies retain
// its identity, while separate calls remain distinct even for equal references.
func NewStateVariable(name string, reference ID) StateVariable {
	return StateVariable{identity: new(int), Name: name, Reference: reference}
}

func (s *StateVariable) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	type plain StateVariable
	v := plain(*s)
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	v.identity = new(int)
	*s = StateVariable(v)
	return nil
}

type EffectScheme struct {
	Effect          Effect   `json:"effect"`
	EffectVariables []string `json:"effectVariables"`
	EntityVariables []string `json:"entityVariables"`
}

func (e Effect) MarshalJSON() ([]byte, error) {
	switch e.Kind {
	case "concrete":
		v := e.Components
		if v == nil {
			v = []EffectComponent{}
		}
		return json.Marshal(struct {
			Kind       string            `json:"kind"`
			Components []EffectComponent `json:"components"`
		}{e.Kind, v})
	case "arrow":
		if e.Result == nil {
			return nil, fmt.Errorf("arrow effect missing result")
		}
		v := e.Params
		if v == nil {
			v = []Effect{}
		}
		return json.Marshal(struct {
			Kind   string   `json:"kind"`
			Params []Effect `json:"params"`
			Result *Effect  `json:"result"`
		}{e.Kind, v, e.Result})
	case "variable":
		if e.Name == "" {
			return nil, fmt.Errorf("effect variable missing name")
		}
		return json.Marshal(struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		}{e.Kind, e.Name})
	}
	return nil, fmt.Errorf("unknown effect kind %q", e.Kind)
}
func (e *Effect) UnmarshalJSON(data []byte) error {
	type plain Effect
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	required := "kind"
	switch v.Kind {
	case "concrete":
		required += " components"
	case "arrow":
		required += " params result"
	case "variable":
		required += " name"
	default:
		return fmt.Errorf("unknown effect kind %q", v.Kind)
	}
	if err := checkObject(data, required, ""); err != nil {
		return err
	}
	if v.Kind == "variable" && v.Name == "" {
		return fmt.Errorf("empty effect variable")
	}
	*e = Effect(v)
	return nil
}
func (e Entity) MarshalJSON() ([]byte, error) {
	obj := map[string]any{"kind": e.Kind}
	switch e.Kind {
	case "concrete":
		v := e.StateVariables
		if v == nil {
			v = []StateVariable{}
		}
		obj["stateVariables"] = v
	case "variable":
		if e.Name == "" {
			return nil, fmt.Errorf("empty entity variable")
		}
		obj["name"] = e.Name
		if e.Reference != nil {
			obj["reference"] = e.Reference
		}
	case "union":
		v := e.Entities
		if v == nil {
			v = []Entity{}
		}
		obj["entities"] = v
	default:
		return nil, fmt.Errorf("unknown entity kind %q", e.Kind)
	}
	return json.Marshal(obj)
}
func (e *Entity) UnmarshalJSON(data []byte) error {
	type plain Entity
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	required, optional := "kind", ""
	switch v.Kind {
	case "concrete":
		required += " stateVariables"
	case "variable":
		required += " name"
		optional = "reference"
	case "union":
		required += " entities"
	default:
		return fmt.Errorf("unknown entity kind %q", v.Kind)
	}
	if err := checkObject(data, required, optional); err != nil {
		return err
	}
	if v.Kind == "variable" && v.Name == "" {
		return fmt.Errorf("empty entity variable")
	}
	*e = Entity(v)
	return nil
}
func (c *EffectComponent) UnmarshalJSON(data []byte) error {
	if err := checkObject(data, "kind entity", ""); err != nil {
		return err
	}
	type plain EffectComponent
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch v.Kind {
	case "read", "update", "temporal":
	default:
		return fmt.Errorf("unknown effect component %q", v.Kind)
	}
	*c = EffectComponent(v)
	return nil
}
