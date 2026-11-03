package quint

import "github.com/glycerine/insdict"

// TypeConstraintSolvingFunction supplies an alternative solver, matching the
// upstream generator's injectable solving function. A nil Solver uses Unifier.
type TypeConstraintSolvingFunction func(Constraint) (ConstraintSolution, error)

// TypeConstraintGenerator accumulates expression types and constraints.
// Input must be name-resolved IR with type applications already expanded.
type TypeConstraintGenerator struct {
	Context     TypeSignatureContext
	Scope       *insdict.Dict[string, TypeScheme]
	Constraints []Constraint
	Solver      TypeConstraintSolvingFunction
}

func NewTypeConstraintGenerator(u TypeUnifier) *TypeConstraintGenerator {
	return &TypeConstraintGenerator{Context: TypeSignatureContext{Unifier: u, Types: insdict.NewDict[string, TypeScheme](), Errors: insdict.NewDict[string, ErrorTree]()}, Scope: insdict.NewDict[string, TypeScheme]()}
}
func (g *TypeConstraintGenerator) add(id ID, t Type) {
	s := TypeScheme{Type: t, TypeVariables: []string{}, RowVariables: []string{}}
	g.Context.Types.Set(id.String(), s)
	g.Scope.Set(id.String(), s)
}
func (g *TypeConstraintGenerator) failed() bool {
	for range g.Context.Errors.All() {
		return true
	}
	return false
}
func (g *TypeConstraintGenerator) record(id ID, e *ErrorTree) {
	if e != nil {
		g.Context.Errors.Set(id.String(), WrapError(g.Context.Location, *e))
	}
}

// Solve applies accumulated constraints and updates every active scheme.
func (g *TypeConstraintGenerator) Solve() ([]Substitution, error) {
	cs := g.Constraints
	g.Constraints = nil
	solver := g.Solver
	if solver == nil {
		solver = g.Context.Unifier.SolveConstraint
	}
	solution, err := solver(Constraint{Kind: "conjunction", Constraints: cs})
	if err != nil {
		return nil, err
	}
	failed := false
	if solution.Errors != nil {
		for id, e := range solution.Errors.All() {
			g.Context.Errors.Set(id, e)
			failed = true
		}
	}
	if failed {
		return nil, nil
	}
	for id, s := range g.Scope.All() {
		t, err := ApplySubstitution(solution.Substitutions, s.Type)
		if err != nil {
			return nil, err
		}
		s.Type = t
		g.Scope.Set(id, s)
		g.Context.Types.Set(id, s)
	}
	return solution.Substitutions, nil
}

// Generate traverses expressions, solving local definitions before their bodies.
func (g *TypeConstraintGenerator) Generate(e *Expr) error {
	if e == nil {
		return nil
	}
	g.Context.Location = "Generating constraints for " + ExpressionString(*e)
	switch e.Kind {
	case "bool", "int", "str":
		g.add(e.ID, Type{Kind: e.Kind})
	case "name":
		if !g.failed() {
			t, err := g.Context.TypeForName(e.Name, e.ID, 2)
			g.record(e.ID, err)
			if err == nil {
				g.add(e.ID, t)
			}
		}
	case "app":
		for i := range e.Args {
			if err := g.Generate(&e.Args[i]); err != nil {
				return err
			}
		}
		if g.failed() {
			return nil
		}
		args := make([]TypedExpression, len(e.Args))
		for i, a := range e.Args {
			args[i] = TypedExpression{Expr: a, Type: g.Context.Types.Get(a.ID.String()).Type}
		}
		signature, signatureError := g.Context.TypeForName(e.Opcode, e.ID, len(e.Args))
		result := Type{Kind: "var", Name: g.Context.Fresh.Next("_t")}
		special := false
		switch e.Opcode {
		case "Rec", "field", "fieldNames", "with", "Tup", "item", "variant", "matchVariant":
			special = true
		}
		if !special && signatureError != nil {
			g.record(e.ID, signatureError)
			return nil
		}
		cs, errors := ApplicationConstraints(e.Opcode, e.ID, args, result, signature)
		if len(errors) > 0 {
			// Special constraints can reject several independent fields/cases.
			// Preserve all leaves under the application instead of replacing an
			// earlier failure at the same expression ID with a later one.
			g.Context.Errors.Set(e.ID.String(), ErrorTree{Location: g.Context.Location, Children: errors})
		}
		if len(errors) == 0 {
			g.Constraints = append(g.Constraints, cs...)
			g.add(e.ID, result)
		}
	case "lambda":
		for _, p := range e.Params {
			name := "t_" + p.Name + "_" + p.ID.String()
			if p.Name == "_" {
				name = g.Context.Fresh.Next("_t")
			}
			typ := Type{Kind: "var", Name: name}
			g.add(p.ID, typ)
			if p.TypeAnnotation != nil {
				g.Constraints = append(g.Constraints, Constraint{Kind: "eq", SourceID: p.ID, Types: [2]Type{typ, *p.TypeAnnotation}})
			}
		}
		if err := g.Generate(e.Body); err != nil {
			return err
		}
		if g.failed() {
			return nil
		}
		args := []Type{}
		for _, p := range e.Params {
			t, err := g.Context.Unifier.InstantiateScheme(g.Context.Types.Get(p.ID.String()), &g.Context.Fresh)
			if err != nil {
				g.record(e.ID, err)
				return nil
			}
			args = append(args, t)
		}
		result := g.Context.Types.Get(e.Body.ID.String()).Type
		g.add(e.ID, Type{Kind: "oper", Args: args, Res: &result})
	case "let":
		schemes := []TypeScheme{}
		for _, s := range g.Scope.All() {
			schemes = append(schemes, s)
		}
		before, err := FreeVariablesInScope(schemes)
		if err != nil {
			return err
		}
		d := e.Opdef
		if err = g.Generate(d.Expr); err != nil {
			return err
		}
		if !g.failed() {
			body := g.Context.Types.Get(d.Expr.ID.String())
			if d.TypeAnnotation != nil {
				g.Constraints = append(g.Constraints, Constraint{Kind: "eq", SourceID: d.ID, Types: [2]Type{body.Type, *d.TypeAnnotation}})
			}
			subs, err := g.Solve()
			if err != nil {
				return err
			}
			if !g.failed() {
				before, err = ExpandFreeVariables(before, subs)
				if err != nil {
					return err
				}
				if d.TypeAnnotation != nil {
					reference := d.ID
					if d.TypeAnnotation.ID != nil {
						reference = *d.TypeAnnotation.ID
					}
					if failure := CheckAnnotationGenerality(subs, *d.TypeAnnotation); failure != nil {
						g.Context.Errors.Set(reference.String(), *failure)
					}
				}
			}
			// Keep an available outer scheme after a descendant constraint fails,
			// matching the partial table exposed by upstream local definitions.
			if _, failed := g.Context.Errors.Get2(d.Expr.ID.String()); failed {
				goto localBody
			}
			schemes = nil
			for _, s := range g.Scope.All() {
				schemes = append(schemes, s)
			}
			after, err := FreeVariablesInScope(schemes)
			if err != nil {
				return err
			}
			body = g.Context.Types.Get(d.Expr.ID.String())
			generalized, err := GeneralizeType(body.Type, after, before)
			if err != nil {
				return err
			}
			g.Context.Types.Set(d.ID.String(), generalized)
			g.Scope.Set(d.ID.String(), generalized)
		}
	localBody:
		if err = g.Generate(e.Body); err != nil {
			return err
		}
		if !g.failed() {
			s := g.Context.Types.Get(e.Body.ID.String())
			g.Context.Types.Set(e.ID.String(), s)
			g.Scope.Set(e.ID.String(), s)
		}
	}
	return nil
}
