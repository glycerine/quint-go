package quint

// RefreshExpressionIDs copies an expression with fresh IDs and extends source
// and analysis tables under the new keys. Existing table entries remain valid.
// Definition/type annotations are handled by the definition refresh stage.
func RefreshExpressionIDs(expression Expr, ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) Expr {
	fresh := func(old ID) ID { return refreshAnalysisID(old, ids, sources, analysis) }
	var visit func(Expr) Expr
	visit = func(e Expr) Expr {
		out := e
		out.ID = fresh(e.ID)
		out.Params = append([]Parameter{}, e.Params...)
		for j := range out.Params {
			out.Params[j].ID = fresh(out.Params[j].ID)
		}
		if e.Opdef != nil {
			d := *e.Opdef
			d.ID = fresh(d.ID)
			body := visit(*d.Expr)
			d.Expr = &body
			out.Opdef = &d
		}
		out.Args = make([]Expr, len(e.Args))
		for j, a := range e.Args {
			out.Args[j] = visit(a)
		}
		if e.Body != nil {
			body := visit(*e.Body)
			out.Body = &body
		}
		return out
	}
	return visit(expression)
}

func refreshAnalysisID(old ID, ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) ID {
	id := ids.Next()
	if source, ok := sources[old]; ok {
		sources[id] = source
	}
	if analysis.Types != nil {
		if v, ok := analysis.Types.Get2(old.String()); ok {
			analysis.Types.Set(id.String(), v)
		}
	}
	if analysis.Effects != nil {
		if v, ok := analysis.Effects.Get2(old.String()); ok {
			analysis.Effects.Set(id.String(), v)
		}
	}
	if analysis.Modes != nil {
		if v, ok := analysis.Modes.Get2(old.String()); ok {
			analysis.Modes.Set(id.String(), v)
		}
	}
	return id
}
