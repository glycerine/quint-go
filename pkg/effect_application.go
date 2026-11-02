package quint

// InferApplication infers an application after its arguments have been visited.
// It updates argument results and retained substitutions as the upstream visitor
// does. Lambda callers expand their free-name sets from the returned bindings.
func (c *EffectSignatureContext) InferApplication(expr Expr, previous []EffectSubstitution) []EffectSubstitution {
	if c.Errors.Len() > 0 {
		return previous
	}
	c.Location = "Trying to infer effect for operator application in " + ExpressionString(expr)
	saveError := func(e *ErrorTree) []EffectSubstitution { c.Save(expr.ID, EffectScheme{}, e); return previous }
	fail := func(err error) []EffectSubstitution { e := ErrorLeaf(c.Location, err.Error()); return saveError(&e) }
	params := []Effect{}
	for _, arg := range expr.Args {
		s, e := c.Fetch(arg.ID)
		if e != nil {
			return saveError(e)
		}
		p, err := InstantiateEffectScheme(s, &c.Fresh)
		if err != nil {
			return fail(err)
		}
		params = append(params, p)
	}
	result := Effect{Kind: "variable", Name: c.Fresh.Next("_e")}
	arrow, err := ApplyEffectSubstitution(previous, Effect{Kind: "arrow", Params: params, Result: &result})
	if err != nil {
		return fail(err)
	}
	signature, e := c.EffectForName(expr.Opcode, expr.ID, len(expr.Args))
	if e != nil {
		return saveError(e)
	}
	signature, err = ApplyEffectSubstitution(previous, signature)
	if err != nil {
		return fail(err)
	}
	bindings, e := UnifyEffects(signature, arrow)
	if e != nil {
		return saveError(e)
	}
	bindings, err = ComposeEffectSubstitutions(previous, bindings)
	if err != nil {
		return fail(err)
	}
	for i, arg := range expr.Args {
		p, err := ApplyEffectSubstitution(bindings, params[i])
		if err != nil {
			return fail(err)
		}
		c.Save(arg.ID, EffectScheme{Effect: p, EffectVariables: []string{}, EntityVariables: []string{}}, nil)
	}
	out, err := ApplyEffectSubstitution(bindings, result)
	if err != nil {
		return fail(err)
	}
	c.Save(expr.ID, EffectScheme{Effect: out, EffectVariables: []string{}, EntityVariables: []string{}}, nil)
	return bindings
}
