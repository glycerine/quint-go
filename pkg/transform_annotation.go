package quint

// TransformDeclarationTypes copies declaration/expression nodes and transforms
// every type annotation and alias body. Unchanged metadata is shared and must
// be treated as immutable. Instance override parameters are not binders here.
func TransformDeclarationTypes(input Declaration, transform func(Type) (Type, error)) (Declaration, error) {
	typ := func(p *Type) (*Type, error) {
		if p == nil {
			return nil, nil
		}
		t, err := transform(*p)
		return &t, err
	}
	var expr func(*Expr) (*Expr, error)
	expr = func(p *Expr) (*Expr, error) {
		if p == nil {
			return nil, nil
		}
		e := *p
		var err error
		if e.Params != nil {
			e.Params = append([]Parameter{}, e.Params...)
			for i := range e.Params {
				e.Params[i].TypeAnnotation, err = typ(e.Params[i].TypeAnnotation)
				if err != nil {
					return nil, err
				}
			}
		}
		if e.Opdef != nil {
			d := *e.Opdef
			d.TypeAnnotation, err = typ(d.TypeAnnotation)
			if err != nil {
				return nil, err
			}
			d.Expr, err = expr(d.Expr)
			if err != nil {
				return nil, err
			}
			e.Opdef = &d
		}
		if e.Args != nil {
			e.Args = append([]Expr{}, e.Args...)
			for i := range e.Args {
				v, err := expr(&e.Args[i])
				if err != nil {
					return nil, err
				}
				e.Args[i] = *v
			}
		}
		e.Body, err = expr(e.Body)
		if err != nil {
			return nil, err
		}
		return &e, nil
	}
	d := input
	var err error
	d.TypeAnnotation, err = typ(d.TypeAnnotation)
	if err != nil {
		return Declaration{}, err
	}
	d.Type, err = typ(d.Type)
	if err != nil {
		return Declaration{}, err
	}
	d.Expr, err = expr(d.Expr)
	if err != nil {
		return Declaration{}, err
	}
	d.Assumption, err = expr(d.Assumption)
	if err != nil {
		return Declaration{}, err
	}
	if d.Overrides != nil {
		d.Overrides = append([]Override{}, d.Overrides...)
		for i := range d.Overrides {
			v, err := expr(&d.Overrides[i].Expr)
			if err != nil {
				return Declaration{}, err
			}
			d.Overrides[i].Expr = *v
		}
	}
	return d, nil
}

// ResolveDeclarations expands applications throughout declarations. Recoverable
// arity diagnostics accumulate in Errors; structural failures return an error.
func (r *TypeApplicationResolver) ResolveDeclarations(input []Declaration) ([]Declaration, error) {
	out := make([]Declaration, len(input))
	for i, d := range input {
		v, err := TransformDeclarationTypes(d, r.Resolve)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}
