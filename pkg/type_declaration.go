package quint

import "github.com/glycerine/insdict"

func (g *TypeConstraintGenerator) scopeVariables() (TypeVariables, error) {
	schemes := []TypeScheme{}
	for _, s := range g.Scope.All() {
		schemes = append(schemes, s)
	}
	return FreeVariablesInScope(schemes)
}
func (g *TypeConstraintGenerator) saveScheme(id ID, s TypeScheme) {
	g.Scope.Set(id.String(), s)
	g.Context.Types.Set(id.String(), s)
}

// GenerateDeclaration visits one dependency-ordered declaration. Expression
// temporaries leave scope afterward, while constants and variables remain.
// Inferred types remain in Context.Types for later references.
func (g *TypeConstraintGenerator) GenerateDeclaration(d Declaration) error {
	saved := insdict.NewDict[string, bool]()
	for id := range g.Scope.All() {
		saved.Set(id, true)
	}
	before, err := g.scopeVariables()
	if err != nil {
		return err
	}
	switch d.Kind {
	case "const", "var":
		g.add(d.ID, *d.TypeAnnotation)
		return nil
	case "def":
		if err = g.Generate(d.Expr); err != nil {
			return err
		}
		if !g.failed() {
			body := g.Context.Types.Get(d.Expr.ID.String())
			if d.TypeAnnotation != nil {
				g.Constraints = append(g.Constraints, Constraint{Kind: "eq", SourceID: d.ID, Types: [2]Type{body.Type, *d.TypeAnnotation}})
			}
			if len(g.Constraints) > 0 {
				subs, err := g.Solve()
				if err != nil {
					return err
				}
				if g.failed() {
					goto cleanup
				}
				before, err = ExpandFreeVariables(before, subs)
				if err != nil {
					return err
				}
				if d.TypeAnnotation != nil {
					reference := d.ID
					if d.TypeAnnotation.ID != nil {
						reference = *d.TypeAnnotation.ID
					}
					if e := CheckAnnotationGenerality(subs, *d.TypeAnnotation); e != nil {
						g.Context.Errors.Set(reference.String(), *e)
					}
				}
			}
			after, err := g.scopeVariables()
			if err != nil {
				return err
			}
			body = g.Context.Types.Get(d.Expr.ID.String())
			scheme, err := GeneralizeType(body.Type, after, before)
			if err != nil {
				return err
			}
			g.saveScheme(d.ID, scheme)
		}
	case "assume":
		if err = g.Generate(d.Assumption); err != nil {
			return err
		}
		if !g.failed() {
			after, err := g.scopeVariables()
			if err != nil {
				return err
			}
			body := g.Context.Types.Get(d.Assumption.ID.String())
			scheme, err := GeneralizeType(body.Type, after, before)
			if err != nil {
				return err
			}
			g.saveScheme(d.ID, scheme)
			g.Constraints = append(g.Constraints, Constraint{Kind: "eq", SourceID: d.ID, Types: [2]Type{body.Type, {Kind: "bool"}}})
		}
	case "instance":
		for i := range d.Overrides {
			if err = g.Generate(&d.Overrides[i].Expr); err != nil {
				return err
			}
		}
		if !g.failed() {
			for _, o := range d.Overrides {
				original, e := g.Context.TypeForName(o.Parameter.Name, o.Parameter.ID, 2)
				g.record(o.Parameter.ID, e)
				if e == nil {
					g.add(o.Parameter.ID, original)
					body := g.Context.Types.Get(o.Expr.ID.String()).Type
					g.Constraints = append(g.Constraints, Constraint{Kind: "eq", SourceID: o.Expr.ID, Types: [2]Type{original, body}})
				}
			}
			if len(g.Constraints) > 0 {
				if _, err = g.Solve(); err != nil {
					return err
				}
			}
		}
	}
cleanup:
	restored := insdict.NewDict[string, TypeScheme]()
	for id, s := range g.Scope.All() {
		if saved.Get(id) {
			restored.Set(id, s)
		}
	}
	g.Scope = restored
	return nil
}
