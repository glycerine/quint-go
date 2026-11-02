package quint

// RefreshDefinitionIDs refreshes definition, annotation, expression, and lambda
// parameter IDs in upstream transformer order, extending retained metadata.
func RefreshDefinitionIDs(def Declaration, ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) (Declaration, error) {
	fresh := func(id ID) ID { return refreshAnalysisID(id, ids, sources, analysis) }
	var definition func(Declaration) (Declaration, error)
	var expression func(Expr) (Expr, error)
	annotation := func(t *Type) (*Type, error) {
		if t == nil {
			return nil, nil
		}
		out, err := RefreshTypeIDs(*t, ids, sources, analysis)
		return &out, err
	}
	definition = func(d Declaration) (Declaration, error) {
		out := d
		out.ID = fresh(d.ID)
		var err error
		out.TypeAnnotation, err = annotation(d.TypeAnnotation)
		if err != nil {
			return Declaration{}, err
		}
		out.Type, err = annotation(d.Type)
		if err != nil {
			return Declaration{}, err
		}
		if d.Expr != nil {
			e, err := expression(*d.Expr)
			if err != nil {
				return Declaration{}, err
			}
			out.Expr = &e
		}
		if d.Assumption != nil {
			e, err := expression(*d.Assumption)
			if err != nil {
				return Declaration{}, err
			}
			out.Assumption = &e
		}
		return out, nil
	}
	expression = func(e Expr) (Expr, error) {
		out := e
		out.ID = fresh(e.ID)
		out.Params = append([]Parameter{}, e.Params...)
		for j := range out.Params {
			out.Params[j].ID = fresh(out.Params[j].ID)
		}
		for j := range out.Params {
			var err error
			out.Params[j].TypeAnnotation, err = annotation(out.Params[j].TypeAnnotation)
			if err != nil {
				return Expr{}, err
			}
		}
		if e.Opdef != nil {
			op := e.Opdef
			d, err := definition(Declaration{ID: op.ID, Kind: "def", Name: op.Name, Qualifier: op.Qualifier, TypeAnnotation: op.TypeAnnotation, Expr: op.Expr})
			if err != nil {
				return Expr{}, err
			}
			copy := *op
			copy.ID = d.ID
			copy.TypeAnnotation = d.TypeAnnotation
			copy.Expr = d.Expr
			out.Opdef = &copy
		}
		out.Args = make([]Expr, len(e.Args))
		for j, a := range e.Args {
			v, err := expression(a)
			if err != nil {
				return Expr{}, err
			}
			out.Args[j] = v
		}
		if e.Body != nil {
			v, err := expression(*e.Body)
			if err != nil {
				return Expr{}, err
			}
			out.Body = &v
		}
		return out, nil
	}
	return definition(def)
}
