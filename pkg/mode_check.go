package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

type ModeChecker struct {
	Errors      *insdict.Dict[string, Diagnostic]
	Suggestions *insdict.Dict[string, Qualifier]
}

func NewModeChecker() *ModeChecker {
	return &ModeChecker{Errors: insdict.NewDict[string, Diagnostic](), Suggestions: insdict.NewDict[string, Qualifier]()}
}
func modeConstraint(q, expected Qualifier) string {
	switch q {
	case PureVal:
		if expected == PureDef {
			return "may not have parameters"
		}
		return "may not interact with state variables"
	case PureDef:
		return "may not interact with state variables"
	case Val:
		if expected == Def {
			return "may not have parameters"
		}
		return "may only read state variables"
	case Def:
		return "may only read state variables"
	case Action:
		return "may only read and update state variables"
	case Temporal:
		return "may not update state variables"
	default:
		return "[not supported by the mode checker]"
	}
}

// CheckDefinition compares one inferred scheme with its annotated qualifier.
// Application-specific action-as-temporal explanations are supplied by traversal.
func (c *ModeChecker) CheckDefinition(id ID, name string, q Qualifier, s EffectScheme) error {
	mode, explanation, err := ModeForEffect(s, q)
	if err != nil {
		return err
	}
	if mode == q {
		return nil
	}
	if !ModeMoreGeneral(mode, q) {
		c.Suggestions.Set(id.String(), mode)
		return nil
	}
	original, replacement := QualifierString(q), QualifierString(mode)
	d := Diagnostic{Code: "QNT200", Message: fmt.Sprintf("%s operators %s, but operator `%s` %s. Use %s instead.", original, modeConstraint(q, mode), name, explanation, replacement), Reference: &id}
	data := DiagnosticData{Fix: &DiagnosticFix{Kind: "replace", Original: original, Replacement: replacement}}
	d.Data = &data
	c.Errors.Set(id.String(), d)
	return nil
}
