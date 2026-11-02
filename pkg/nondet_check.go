package quint

import "github.com/glycerine/insdict"

// NondetChecker retains diagnostics and the last visited operator qualifier
// across batches, matching upstream visitor state (not a lexical qualifier stack).
type NondetChecker struct {
	Table  *insdict.Dict[ID, LookupDefinition]
	Errors []Diagnostic
	last   Qualifier
}

func NewNondetChecker(table *insdict.Dict[ID, LookupDefinition]) *NondetChecker {
	return &NondetChecker{Table: table, Errors: []Diagnostic{}, last: Def}
}
func (c *NondetChecker) report(code, message string, id ID) {
	c.Errors = append(c.Errors, Diagnostic{Code: code, Message: message, Reference: &id, Data: &DiagnosticData{}})
}
func (c *NondetChecker) enterDef(id ID, q Qualifier) {
	c.last = q
	if c.Table != nil {
		if entry, ok := c.Table.Get2(id); ok && q == Nondet && entry.Depth == 0 {
			c.report(QNT206, "'nondet' can only be used inside actions, not at the top level", id)
		}
	}
}

// Check validates name-resolved declarations using their inferred type table.
func (c *NondetChecker) Check(types *insdict.Dict[string, TypeScheme], declarations []Declaration) ([]Diagnostic, error) {
	var expr func(*Expr) error
	expr = func(e *Expr) error {
		if e == nil {
			return nil
		}
		if e.Kind == "app" && (e.Opcode == "oneOf" || e.Opcode == "apalache::generate") && c.last != Nondet {
			c.report(QNT203, "'"+e.Opcode+"' must be used inside a nondet definition", e.ID)
		}
		if e.Kind == "let" {
			d := e.Opdef
			if d.Qualifier == Nondet {
				if d.Expr.Kind != "app" || (d.Expr.Opcode != "oneOf" && d.Expr.Opcode != "apalache::generate") {
					c.report(QNT204, "the outermost expression in a nondet definition must be either 'oneOf' or 'apalache::generate'", d.Expr.ID)
				}
				if types != nil {
					if s, ok := types.Get2(e.Body.ID.String()); ok && s.Type.Kind != "bool" {
						text, failure := TypeSchemeString(s)
						if failure != nil {
							return failureAsError{*failure}
						}
						c.report(QNT205, "nondet bindings can only be used with boolean expressions, but expression has type: "+text, e.ID)
					}
				}
			}
			c.enterDef(d.ID, d.Qualifier)
			if err := expr(d.Expr); err != nil {
				return err
			}
		}
		for i := range e.Args {
			if err := expr(&e.Args[i]); err != nil {
				return err
			}
		}
		return expr(e.Body)
	}
	for _, d := range declarations {
		if d.Kind == "def" {
			c.enterDef(d.ID, d.Qualifier)
		}
		if err := expr(d.Expr); err != nil {
			return c.Errors, err
		}
		if err := expr(d.Assumption); err != nil {
			return c.Errors, err
		}
		for i := range d.Overrides {
			if err := expr(&d.Overrides[i].Expr); err != nil {
				return c.Errors, err
			}
		}
	}
	return c.Errors, nil
}

type failureAsError struct{ ErrorTree }

func (e failureAsError) Error() string { return e.String() }
