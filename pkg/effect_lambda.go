package quint

// EffectLambdaScopes holds the nested free-name sets used by effect inference.
type EffectLambdaScopes struct{ Frames []EffectFreeNames }

func (s *EffectLambdaScopes) Current() EffectFreeNames {
	if len(s.Frames) == 0 {
		return EffectFreeNames{}
	}
	return s.Frames[len(s.Frames)-1]
}
func (s *EffectLambdaScopes) Expand(bindings []EffectSubstitution) {
	for i := range s.Frames {
		s.Frames[i] = ExpandEffectFreeNames(s.Frames[i], bindings)
	}
}
func (c *EffectSignatureContext) EnterEffectLambda(expr Expr, scopes *EffectLambdaScopes) {
	previous := scopes.Current()
	free := EffectFreeNames{Effects: append([]string{}, previous.Effects...), Entities: append([]string{}, previous.Entities...)}
	for _, p := range expr.Params {
		name := "e_" + p.Name + "_" + p.ID.String()
		if p.Name == "_" {
			name = c.Fresh.Next("_e")
		}
		free.Effects = appendEffectName(free.Effects, name)
		c.Save(p.ID, EffectScheme{Effect: Effect{Kind: "variable", Name: name}, EffectVariables: []string{}, EntityVariables: []string{}}, nil)
	}
	scopes.Frames = append(scopes.Frames, free)
}
func (c *EffectSignatureContext) ExitEffectLambda(expr Expr, scopes *EffectLambdaScopes, bindings []EffectSubstitution) {
	// Upstream retains the frame on failure because inference stops after errors.
	if c.Errors.Len() > 0 {
		return
	}
	scopes.Expand(bindings)
	body, e := c.Fetch(expr.Body.ID)
	if e != nil {
		c.Save(expr.ID, EffectScheme{}, e)
		return
	}
	params := []Effect{}
	for _, p := range expr.Params {
		scheme, e := c.Fetch(p.ID)
		if e != nil {
			c.Save(expr.ID, EffectScheme{}, e)
			return
		}
		effect, err := InstantiateEffectScheme(scheme, &c.Fresh)
		if err == nil {
			effect, err = ApplyEffectSubstitution(bindings, effect)
		}
		if err != nil {
			v := ErrorLeaf(c.Location, err.Error())
			c.Save(p.ID, EffectScheme{}, &v)
			return
		}
		c.Save(p.ID, EffectScheme{Effect: effect, EffectVariables: []string{}, EntityVariables: []string{}}, nil)
		params = append(params, effect)
	}
	result := body.Effect
	body.Effect = Effect{Kind: "arrow", Params: params, Result: &result}
	out, err := InstantiateEffectScheme(body, &c.Fresh)
	if err == nil {
		out, err = ApplyEffectSubstitution(bindings, out)
	}
	if err != nil {
		v := ErrorLeaf(c.Location, err.Error())
		c.Save(expr.ID, EffectScheme{}, &v)
		return
	}
	if out.Result.Kind == "arrow" {
		v := ErrorLeaf(c.Location, "Result cannot be an operator")
		c.Save(expr.Body.ID, EffectScheme{}, &v)
	}
	c.Save(expr.ID, EffectScheme{Effect: out, EffectVariables: []string{}, EntityVariables: []string{}}, nil)
	scopes.Frames = scopes.Frames[:len(scopes.Frames)-1]
}
