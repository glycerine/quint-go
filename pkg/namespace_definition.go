package quint

import "strings"

// AddNamespaceToDefinition qualifies a definition for flattening. Definition
// and parameter names ignore preserve; reference and constant-type names honor it.
func AddNamespaceToDefinition(d Declaration, namespace string, preserve map[string]bool) (Declaration, error) {
	name := func(n string) string {
		if n == "_" || namespace == "" || strings.HasPrefix(n, namespace+"::") {
			return n
		}
		return namespace + "::" + n
	}
	out, err := TransformDeclarationTypes(d, func(t Type) (Type, error) {
		copy, err := ApplySubstitution(nil, t)
		if err != nil {
			return Type{}, err
		}
		err = WalkType(&copy, func(t *Type) (bool, error) {
			if t.Kind == "const" && !preserve[t.Name] {
				t.Name = name(t.Name)
			}
			return true, nil
		})
		return copy, err
	})
	if err != nil {
		return Declaration{}, err
	}
	var expr func(Expr) Expr
	expr = func(e Expr) Expr {
		out := e
		if e.Kind == "name" && !preserve[e.Name] {
			out.Name = name(e.Name)
		}
		if e.Kind == "app" && !preserve[e.Opcode] {
			out.Opcode = name(e.Opcode)
		}
		out.Params = append([]Parameter{}, e.Params...)
		for i := range out.Params {
			out.Params[i].Name = name(out.Params[i].Name)
		}
		out.Args = make([]Expr, len(e.Args))
		for i, a := range e.Args {
			out.Args[i] = expr(a)
		}
		if e.Body != nil {
			b := expr(*e.Body)
			out.Body = &b
		}
		if e.Opdef != nil {
			op := *e.Opdef
			op.Name = name(op.Name)
			b := expr(*op.Expr)
			op.Expr = &b
			out.Opdef = &op
		}
		return out
	}
	out.Name = name(out.Name)
	if out.Expr != nil {
		e := expr(*out.Expr)
		out.Expr = &e
	}
	if out.Assumption != nil {
		e := expr(*out.Assumption)
		out.Assumption = &e
	}
	return out, nil
}
