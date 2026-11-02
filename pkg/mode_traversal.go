package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

// CheckModes visits declarations and local definitions in upstream exit order.
// Errors and suggestions accumulate across calls.
func (c *ModeChecker) CheckModes(declarations []Declaration, effects *insdict.Dict[string, EffectScheme]) error {
	var expression func(Expr) error
	definition := func(id ID, name string, q Qualifier) error {
		if s, ok := effects.Get2(id.String()); ok {
			return c.CheckDefinition(id, name, q, s)
		}
		return nil
	}
	expression = func(e Expr) error {
		if e.Kind == "let" {
			if err := expression(*e.Opdef.Expr); err != nil {
				return err
			}
			if err := definition(e.Opdef.ID, e.Opdef.Name, e.Opdef.Qualifier); err != nil {
				return err
			}
		}
		for _, a := range e.Args {
			if err := expression(a); err != nil {
				return err
			}
		}
		if e.Body != nil {
			return expression(*e.Body)
		}
		return nil
	}
	for _, d := range declarations {
		if d.Expr != nil {
			if err := expression(*d.Expr); err != nil {
				return err
			}
		}
		if d.Assumption != nil {
			if err := expression(*d.Assumption); err != nil {
				return err
			}
		}
		for _, o := range d.Overrides {
			if err := expression(o.Expr); err != nil {
				return err
			}
		}
		if d.Kind == "def" {
			if err := definition(d.ID, d.Name, d.Qualifier); err != nil {
				return err
			}
		}
		if d.Kind == "instance" {
			for _, o := range d.Overrides {
				s, ok := effects.Get2(o.Expr.ID.String())
				if !ok {
					continue
				}
				mode, explanation, err := ModeForEffect(s, PureDef)
				if err != nil {
					return err
				}
				if mode == PureVal || mode == PureDef {
					continue
				}
				id := o.Expr.ID
				c.Errors.Set(id.String(), Diagnostic{Code: "QNT201", Message: fmt.Sprintf("Instance overrides must be pure, but the value for %s %s", o.Parameter.Name, explanation), Reference: &id, Data: &DiagnosticData{}})
			}
		}
	}
	return nil
}
