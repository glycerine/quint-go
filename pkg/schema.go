package quint

import (
	"encoding/json"
	"fmt"
	"strings"
)

// checkObject enforces a tagged object's field set before typed decoding, which
// otherwise silently accepts misspelled fields and absent zero-valued members.
func checkObject(data []byte, required, optional string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("expected JSON object")
	}
	allowed := map[string]bool{}
	for _, key := range strings.Fields(required) {
		allowed[key] = true
		value, ok := obj[key]
		if !ok || string(value) == "null" {
			return fmt.Errorf("missing required field %q", key)
		}
	}
	for _, key := range strings.Fields(optional) {
		allowed[key] = true
	}
	for key, value := range obj {
		if !allowed[key] {
			return fmt.Errorf("unexpected field %q", key)
		}
		if string(value) == "null" {
			return fmt.Errorf("null field %q", key)
		}
	}
	return nil
}
func declarationSchema(data []byte) error {
	var tag struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	required, optional := "id kind", "doc depth hidden namespaces shadowing importedFrom"
	switch tag.Kind {
	case "def":
		required += " name qualifier expr"
		optional += " typeAnnotation"
	case "const", "var":
		required += " name typeAnnotation"
	case "assume":
		required += " name assumption"
	case "typedef":
		required += " name"
		optional += " params type"
	case "import":
		required += " protoName"
		optional += " defName qualifiedName fromSource"
	case "export":
		required += " protoName"
		optional += " defName qualifiedName"
	case "instance":
		required += " protoName overrides identityOverride"
		optional += " qualifiedName fromSource"
	default:
		return fmt.Errorf("unknown declaration kind %q", tag.Kind)
	}
	return checkObject(data, required, optional)
}
func (m *Module) UnmarshalJSON(data []byte) error {
	if err := checkObject(data, "id name declarations", "doc"); err != nil {
		return err
	}
	type plain Module
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Name == "" {
		return fmt.Errorf("empty module name")
	}
	*m = Module(p)
	return nil
}
func (p *Parameter) UnmarshalJSON(data []byte) error {
	if err := checkObject(data, "id name", "typeAnnotation depth kind shadowing"); err != nil {
		return err
	}
	type plain Parameter
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.Name == "" {
		return fmt.Errorf("empty parameter name")
	}
	*p = Parameter(v)
	return nil
}
func (d *OperatorDef) UnmarshalJSON(data []byte) error {
	if err := declarationSchema(data); err != nil {
		return err
	}
	type plain OperatorDef
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Kind != "def" || p.Name == "" || !p.Qualifier.Valid() {
		return fmt.Errorf("invalid operator definition")
	}
	*d = OperatorDef(p)
	return nil
}
