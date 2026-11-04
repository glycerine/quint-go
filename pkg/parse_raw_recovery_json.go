package quint

import (
	"encoding/json"
	"fmt"
)

// This tag is confined to the parser's private raw replay. It is never a valid
// Expr variant and has no expression ID in the upstream listener's output.
const rawRecoveryNullKind = "\x00parser-null"

// marshalRawRecoveredModules serializes the listener's partial IR without
// admitting missing operands into the strict public expression wire format.
func marshalRawRecoveredModules(modules []Module) (json.RawMessage, error) {
	var wrapped []rawRecoveryModule
	if modules != nil {
		wrapped = make([]rawRecoveryModule, len(modules))
		for i, module := range modules {
			wrapped[i] = rawRecoveryModule{module}
		}
	}
	data, err := json.Marshal(wrapped)
	return json.RawMessage(data), err
}

type rawRecoveryModule struct{ module Module }
type rawRecoveryModuleFields Module

func (m rawRecoveryModule) MarshalJSON() ([]byte, error) {
	var declarations []rawRecoveryDeclaration
	if m.module.Declarations != nil {
		declarations = make([]rawRecoveryDeclaration, len(m.module.Declarations))
		for i, declaration := range m.module.Declarations {
			declarations[i] = rawRecoveryDeclaration{declaration}
		}
	}
	return json.Marshal(struct {
		rawRecoveryModuleFields
		Declarations []rawRecoveryDeclaration `json:"declarations"`
	}{rawRecoveryModuleFields(m.module), declarations})
}

type rawRecoveryDeclaration struct{ declaration Declaration }
type rawRecoveryDeclarationFields Declaration

func (d rawRecoveryDeclaration) MarshalJSON() ([]byte, error) {
	declaration := d.declaration
	if err := declaration.Validate(); err != nil {
		return nil, err
	}
	var overrides []rawRecoveryOverride
	if declaration.Overrides != nil {
		overrides = make([]rawRecoveryOverride, len(declaration.Overrides))
		for i, override := range declaration.Overrides {
			overrides[i] = rawRecoveryOverride{override}
		}
	}
	data, err := json.Marshal(struct {
		rawRecoveryDeclarationFields
		Expr       *rawRecoveryExpr      `json:"expr,omitempty"`
		Assumption *rawRecoveryExpr      `json:"assumption,omitempty"`
		Overrides  []rawRecoveryOverride `json:"overrides,omitempty"`
	}{rawRecoveryDeclarationFields(declaration), rawRecoveryExprPointer(declaration.Expr), rawRecoveryExprPointer(declaration.Assumption), overrides})
	if err != nil {
		return nil, err
	}
	// Match Declaration.MarshalJSON's explicitly present empty metadata.
	preserveEmptyNamespaces := declaration.Namespaces != nil && len(declaration.Namespaces) == 0
	if declaration.Kind != "instance" && !(declaration.Kind == "typedef" && declaration.Params != nil) && !preserveEmptyNamespaces {
		return data, nil
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if preserveEmptyNamespaces {
		object["namespaces"] = json.RawMessage("[]")
	}
	if declaration.Kind == "typedef" && declaration.Params != nil {
		object["params"], err = json.Marshal(declaration.Params)
		if err != nil {
			return nil, err
		}
	}
	if declaration.Kind == "instance" {
		if overrides == nil {
			overrides = []rawRecoveryOverride{}
		}
		object["overrides"], err = json.Marshal(overrides)
		if err != nil {
			return nil, err
		}
		object["identityOverride"], err = json.Marshal(declaration.IdentityOverride)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(object)
}

type rawRecoveryOverride struct{ override Override }

func (o rawRecoveryOverride) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]any{o.override.Parameter, rawRecoveryExpr{o.override.Expr}})
}

type rawRecoveryOperator struct{ operator OperatorDef }
type rawRecoveryOperatorFields OperatorDef

func (o rawRecoveryOperator) MarshalJSON() ([]byte, error) {
	if o.operator.Kind != "def" {
		return nil, fmt.Errorf("unknown operator definition kind %q", o.operator.Kind)
	}
	return json.Marshal(struct {
		rawRecoveryOperatorFields
		Expr *rawRecoveryExpr `json:"expr"`
	}{rawRecoveryOperatorFields(o.operator), rawRecoveryExprPointer(o.operator.Expr)})
}

type rawRecoveryExpr struct{ expression Expr }

func rawRecoveryExprPointer(expression *Expr) *rawRecoveryExpr {
	if expression == nil {
		return nil
	}
	return &rawRecoveryExpr{*expression}
}

func (r rawRecoveryExpr) MarshalJSON() ([]byte, error) {
	expression := r.expression
	switch expression.Kind {
	case rawRecoveryNullKind:
		return []byte("null"), nil
	case "name", "bool", "int", "str":
		return expression.MarshalJSON()
	case "app":
		args := make([]rawRecoveryExpr, len(expression.Args))
		for i, arg := range expression.Args {
			args[i] = rawRecoveryExpr{arg}
		}
		return json.Marshal(map[string]any{"id": expression.ID, "kind": expression.Kind, "opcode": expression.Opcode, "args": args})
	case "lambda":
		if expression.Body == nil || !expression.Qualifier.Valid() {
			return nil, fmt.Errorf("invalid lambda")
		}
		params := expression.Params
		if params == nil {
			params = []Parameter{}
		}
		return json.Marshal(map[string]any{"id": expression.ID, "kind": expression.Kind, "qualifier": expression.Qualifier,
			"params": params, "expr": rawRecoveryExprPointer(expression.Body)})
	case "let":
		if expression.Body == nil || expression.Opdef == nil {
			return nil, fmt.Errorf("invalid let")
		}
		return json.Marshal(map[string]any{"id": expression.ID, "kind": expression.Kind,
			"opdef": rawRecoveryOperator{*expression.Opdef}, "expr": rawRecoveryExprPointer(expression.Body)})
	default:
		return nil, fmt.Errorf("unknown expression kind %q", expression.Kind)
	}
}
