package quint

import "fmt"

// IRDefinition borrows either a top-level definition or a local let operator.
// Exactly one field is set in a valid reference. The referenced IR remains
// caller-owned and must not be mutated during traversal or through this view.
type IRDefinition struct {
	Declaration *Declaration
	Operator    *OperatorDef
}

func (d IRDefinition) Valid() bool { return (d.Declaration == nil) != (d.Operator == nil) }
func (d IRDefinition) ID() ID {
	if d.Declaration != nil {
		return d.Declaration.ID
	}
	if d.Operator != nil {
		return d.Operator.ID
	}
	return ID{}
}
func (d IRDefinition) Kind() string {
	if d.Declaration != nil {
		return d.Declaration.Kind
	}
	if d.Operator != nil {
		return "def"
	}
	return ""
}

// IRVisitor walks borrowed IR in upstream order, including annotations, rows,
// instance override expressions, and local definitions. General enter/exit
// callbacks inspect Kind to distinguish node variants. Parameter callbacks are
// an additional Go hook around each lambda parameter's annotation; instance
// override parameters are not visited, matching upstream.
//
// The zero value and nil callbacks are usable. DefinitionDepth follows the
// upstream definition/let/lambda depth rules; WalkDeclaration initializes it.
// Set it to -1 before WalkDefinition to visit a standalone definition at depth
// zero. A callback error stops traversal without pending exit callbacks and
// restores the entry depth. One visitor and its callbacks are used sequentially;
// callbacks must not mutate or retain mutable access to the borrowed tree.
type IRVisitor struct {
	DefinitionDepth                   int
	EnterModule, ExitModule           func(*Module) error
	EnterDeclaration, ExitDeclaration func(*Declaration) error
	EnterDefinition, ExitDefinition   func(IRDefinition) error
	EnterExpression, ExitExpression   func(*Expr) error
	EnterType, ExitType               func(*Type) error
	EnterRow, ExitRow                 func(*Row) error
	EnterParameter, ExitParameter     func(*Parameter) error
}

func irAccessCall[T any](callback func(T) error, node T) error {
	if callback != nil {
		return callback(node)
	}
	return nil
}

func (v *IRVisitor) walk(body func(*IRVisitor) error) error {
	if v == nil {
		v = &IRVisitor{}
	}
	depth := v.DefinitionDepth
	err := body(v)
	if err != nil {
		v.DefinitionDepth = depth
	}
	return err
}

func (v *IRVisitor) WalkModule(module *Module) error {
	return v.walk(func(v *IRVisitor) error {
		if module == nil {
			return nil
		}
		if err := irAccessCall(v.EnterModule, module); err != nil {
			return err
		}
		for i := range module.Declarations {
			if err := v.WalkDeclaration(&module.Declarations[i]); err != nil {
				return err
			}
		}
		return irAccessCall(v.ExitModule, module)
	})
}

func (v *IRVisitor) WalkDeclaration(declaration *Declaration) error {
	return v.walk(func(v *IRVisitor) error {
		if declaration == nil {
			return nil
		}
		if err := irAccessCall(v.EnterDeclaration, declaration); err != nil {
			return err
		}
		v.DefinitionDepth = 0
		switch declaration.Kind {
		case "def", "const", "var", "typedef", "assume":
			if declaration.Kind == "def" {
				v.DefinitionDepth = -1
			}
			if err := v.WalkDefinition(IRDefinition{Declaration: declaration}); err != nil {
				return err
			}
		case "instance":
			for i := range declaration.Overrides {
				if err := v.WalkExpression(&declaration.Overrides[i].Expr); err != nil {
					return err
				}
			}
		case "import", "export":
		default:
			return fmt.Errorf("unknown declaration kind %q", declaration.Kind)
		}
		return irAccessCall(v.ExitDeclaration, declaration)
	})
}

func (v *IRVisitor) WalkDefinition(definition IRDefinition) error {
	return v.walk(func(v *IRVisitor) error {
		if !definition.Valid() {
			return fmt.Errorf("IR definition must reference exactly one declaration or local operator")
		}
		v.DefinitionDepth++
		if err := irAccessCall(v.EnterDefinition, definition); err != nil {
			return err
		}
		var annotation, alias *Type
		var expression *Expr
		if d := definition.Declaration; d != nil {
			annotation, alias = d.TypeAnnotation, d.Type
			expression = d.Expr
			if d.Kind == "assume" {
				expression = d.Assumption
			}
		} else {
			annotation, expression = definition.Operator.TypeAnnotation, definition.Operator.Expr
		}
		if annotation != nil {
			if err := v.WalkType(annotation); err != nil {
				return err
			}
		} else if definition.Kind() == "typedef" && alias != nil {
			if err := v.WalkType(alias); err != nil {
				return err
			}
		}
		switch definition.Kind() {
		case "def", "assume":
			if expression == nil {
				return fmt.Errorf("%s definition missing expression", definition.Kind())
			}
			if err := v.WalkExpression(expression); err != nil {
				return err
			}
		case "const", "var", "typedef":
		default:
			return fmt.Errorf("unknown definition kind %q", definition.Kind())
		}
		if err := irAccessCall(v.ExitDefinition, definition); err != nil {
			return err
		}
		v.DefinitionDepth--
		return nil
	})
}

func (v *IRVisitor) WalkExpression(expression *Expr) error {
	return v.walk(func(v *IRVisitor) error {
		if expression == nil {
			return nil
		}
		if err := irAccessCall(v.EnterExpression, expression); err != nil {
			return err
		}
		switch expression.Kind {
		case "name", "bool", "int", "str":
		case "app":
			for i := range expression.Args {
				if err := v.WalkExpression(&expression.Args[i]); err != nil {
					return err
				}
			}
		case "lambda", "let":
			if expression.Body == nil {
				return fmt.Errorf("%s expression missing body", expression.Kind)
			}
			v.DefinitionDepth++
			if expression.Kind == "lambda" {
				for i := range expression.Params {
					parameter := &expression.Params[i]
					if err := irAccessCall(v.EnterParameter, parameter); err != nil {
						return err
					}
					if err := v.WalkType(parameter.TypeAnnotation); err != nil {
						return err
					}
					if err := irAccessCall(v.ExitParameter, parameter); err != nil {
						return err
					}
				}
			} else if err := v.WalkDefinition(IRDefinition{Operator: expression.Opdef}); err != nil {
				return err
			}
			if err := v.WalkExpression(expression.Body); err != nil {
				return err
			}
			v.DefinitionDepth--
		default:
			return fmt.Errorf("unknown expression kind %q", expression.Kind)
		}
		return irAccessCall(v.ExitExpression, expression)
	})
}

func (v *IRVisitor) WalkType(typ *Type) error {
	return v.walk(func(v *IRVisitor) error {
		if typ == nil {
			return nil
		}
		if err := irAccessCall(v.EnterType, typ); err != nil {
			return err
		}
		var children []*Type
		switch typ.Kind {
		case "bool", "int", "str", "const", "var":
		case "set", "list":
			children = []*Type{typ.Elem}
		case "fun":
			children = []*Type{typ.Arg, typ.Res}
		case "oper", "app":
			if typ.Kind == "app" {
				children = append(children, typ.Ctor)
			}
			for i := range typ.Args {
				children = append(children, &typ.Args[i])
			}
			if typ.Kind == "oper" {
				children = append(children, typ.Res)
			}
		case "tup", "rec", "sum":
			if typ.Fields == nil {
				return fmt.Errorf("%s type missing row", typ.Kind)
			}
			if err := v.WalkRow(typ.Fields); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown type kind %q", typ.Kind)
		}
		for _, child := range children {
			if child == nil {
				return fmt.Errorf("%s type missing child", typ.Kind)
			}
			if err := v.WalkType(child); err != nil {
				return err
			}
		}
		return irAccessCall(v.ExitType, typ)
	})
}

func (v *IRVisitor) WalkRow(row *Row) error {
	return v.walk(func(v *IRVisitor) error {
		if row == nil {
			return nil
		}
		if err := irAccessCall(v.EnterRow, row); err != nil {
			return err
		}
		switch row.Kind {
		case "empty", "var":
		case "row":
			for i := range row.Fields {
				if err := v.WalkType(&row.Fields[i].Type); err != nil {
					return err
				}
			}
			if row.Other == nil {
				return fmt.Errorf("concrete row missing tail")
			}
			if err := v.WalkRow(row.Other); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown row kind %q", row.Kind)
		}
		return irAccessCall(v.ExitRow, row)
	})
}

// Module-wide finders borrow nodes from modules and return nil (or an invalid
// IRDefinition) when absent. As upstream does, duplicate IDs select the last
// postorder match. Malformed trees return an error instead of a partial result.
// Inputs and returned nodes must remain read-only while borrowed.
func FindExpression(modules []Module, id ID) (*Expr, error) {
	var found *Expr
	v := IRVisitor{ExitExpression: func(e *Expr) error {
		if e.ID == id {
			found = e
		}
		return nil
	}}
	if err := irAccessModules(modules, &v); err != nil {
		return nil, err
	}
	return found, nil
}

func FindType(modules []Module, id ID) (*Type, error) {
	var found *Type
	v := IRVisitor{ExitType: func(t *Type) error {
		if t.ID != nil && *t.ID == id {
			found = t
		}
		return nil
	}}
	if err := irAccessModules(modules, &v); err != nil {
		return nil, err
	}
	return found, nil
}

func FindDefinition(modules []Module, id ID) (IRDefinition, error) {
	var found IRDefinition
	v := IRVisitor{ExitDefinition: func(d IRDefinition) error {
		if d.ID() == id {
			found = d
		}
		return nil
	}}
	if err := irAccessModules(modules, &v); err != nil {
		return IRDefinition{}, err
	}
	return found, nil
}

// FindParameter finds lambda parameters; instance override parameters are not
// part of the upstream parameter finder. Parameters are considered on lambda
// exit, so an outer lambda wins over a nested lambda with a duplicate ID.
func FindParameter(modules []Module, id ID) (*Parameter, error) {
	var found *Parameter
	v := IRVisitor{ExitExpression: func(e *Expr) error {
		if e.Kind == "lambda" {
			for i := range e.Params {
				if e.Params[i].ID == id {
					found = &e.Params[i]
				}
			}
		}
		return nil
	}}
	if err := irAccessModules(modules, &v); err != nil {
		return nil, err
	}
	return found, nil
}

func irAccessModules(modules []Module, visitor *IRVisitor) error {
	for i := range modules {
		if err := visitor.WalkModule(&modules[i]); err != nil {
			return err
		}
	}
	return nil
}
