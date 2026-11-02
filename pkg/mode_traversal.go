package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

// CheckModes visits declarations and local definitions in upstream exit order.
// Errors and suggestions accumulate across calls.
func (c *ModeChecker) CheckModes(declarations []Declaration, effects *insdict.Dict[string, EffectScheme]) error {
	var expression func(Expr) error
	stack := []*Expr{}
	hasUpdates := func(id ID) bool {
		s, ok := effects.Get2(id.String())
		if !ok || s.Effect.Kind != "concrete" {
			return false
		}
		for _, c := range s.Effect.Components {
			if c.Kind == "update" && (len(EntityNames(c.Entity)) > 0 || entityHasState(c.Entity)) {
				return true
			}
		}
		return false
	}
	definition := func(id ID, name string, q Qualifier) error {
		if s, ok := effects.Get2(id.String()); ok {
			mode, _, err := ModeForEffect(s, q)
			if err != nil {
				return err
			}
			var app *Expr
			if len(stack) > 0 {
				app = stack[len(stack)-1]
			}
			if mode != q && mode == Temporal && app != nil && q != Run {
				hint := ""
				switch app.Opcode {
				case "and":
					hint = " To combine actions, use `all { ... }` instead."
				case "or":
					hint = " For alternative actions, use `any { ... }` instead."
				case "exists":
					hint = " To pick a value non-deterministically in an action, use `nondet x = S.oneOf()` instead."
				}
				ref := app.ID
				c.Errors.Set(ref.String(), Diagnostic{Code: "QNT200", Message: fmt.Sprintf("Using an action as an argument of `%s` is only allowed in temporal definitions, but it is used in %s `%s`.%s", app.Opcode, QualifierString(q), name, hint), Reference: &ref, Data: &DiagnosticData{}})
				return nil
			}
			return c.CheckDefinition(id, name, q, s)
		}
		return nil
	}
	expression = func(e Expr) error {
		if e.Kind == "app" && len(stack) > 0 && stack[len(stack)-1] == nil {
			temporal := false
			switch e.Opcode {
			case "always", "eventually", "next", "orKeep", "mustChange", "enabled", "weakFair", "strongFair", "leadsTo":
				temporal = true
			}
			if !temporal && !hasUpdates(e.ID) {
				for _, a := range e.Args {
					id := a.ID
					if a.Kind == "lambda" {
						id = a.Body.ID
					}
					if hasUpdates(id) {
						copy := e
						stack[len(stack)-1] = &copy
						break
					}
				}
			}
		}
		if e.Kind == "let" {
			stack = append(stack, nil)
			if err := expression(*e.Opdef.Expr); err != nil {
				return err
			}
			if err := definition(e.Opdef.ID, e.Opdef.Name, e.Opdef.Qualifier); err != nil {
				return err
			}
			stack = stack[:len(stack)-1]
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
		if d.Kind == "def" {
			stack = append(stack, nil)
		}
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
		if d.Kind == "def" {
			stack = stack[:len(stack)-1]
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

func entityHasState(e Entity) bool {
	if len(e.StateVariables) > 0 {
		return true
	}
	for _, c := range e.Entities {
		if entityHasState(c) {
			return true
		}
	}
	return false
}
