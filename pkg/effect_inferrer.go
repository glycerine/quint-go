package quint

import "github.com/glycerine/insdict"

type EffectInferrer struct {
	*EffectSignatureContext
	Scopes        EffectLambdaScopes
	Substitutions []EffectSubstitution
}

func NewEffectInferrer(table *insdict.Dict[ID, LookupDefinition]) *EffectInferrer {
	return &EffectInferrer{EffectSignatureContext: NewEffectSignatureContext(table)}
}
func (i *EffectInferrer) inferDefinition(id ID, body *Expr, top bool) {
	i.InferExpression(*body)
	if i.Errors.Len() > 0 {
		return
	}
	s, e := i.Fetch(body.ID)
	if e == nil {
		s = GeneralizeEffect(s.Effect, i.Scopes.Current())
	}
	i.Save(id, s, e)
	if top {
		i.Substitutions = nil
	}
}

// InferExpression visits a valid expression in upstream visitor order.
func (i *EffectInferrer) InferExpression(e Expr) {
	i.Location = "Inferring effect for " + ExpressionString(e)
	switch e.Kind {
	case "bool", "int", "str":
		i.Save(e.ID, EffectScheme{Effect: Effect{Kind: "concrete", Components: []EffectComponent{}}, EffectVariables: []string{}, EntityVariables: []string{}}, nil)
	case "name":
		if i.Errors.Len() > 0 {
			return
		}
		effect, err := i.EffectForName(e.Name, e.ID, 2)
		i.Save(e.ID, EffectScheme{Effect: effect, EffectVariables: []string{}, EntityVariables: []string{}}, err)
	case "app":
		for _, a := range e.Args {
			i.InferExpression(a)
		}
		i.Substitutions = i.InferApplication(e, i.Substitutions)
		i.Scopes.Expand(i.Substitutions)
	case "lambda":
		i.EnterEffectLambda(e, &i.Scopes)
		i.InferExpression(*e.Body)
		i.ExitEffectLambda(e, &i.Scopes, i.Substitutions)
	case "let":
		i.inferDefinition(e.Opdef.ID, e.Opdef.Expr, false)
		i.InferExpression(*e.Body)
		if i.Errors.Len() == 0 {
			s, err := i.Fetch(e.Body.ID)
			i.Save(e.ID, s, err)
		}
	}
}

// InferEffects retains tables across declaration batches. Names must already be
// resolved and declarations dependency ordered.
func (i *EffectInferrer) InferEffects(declarations []Declaration) {
	for _, d := range declarations {
		if i.SeedDeclaration(d) {
			continue
		}
		switch d.Kind {
		case "def":
			i.inferDefinition(d.ID, d.Expr, true)
		case "assume":
			i.InferExpression(*d.Assumption)
		case "instance":
			for _, o := range d.Overrides {
				i.InferExpression(o.Expr)
			}
		}
	}
}
