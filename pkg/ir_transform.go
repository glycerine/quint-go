package quint

import (
	"fmt"
	"slices"
)

// IRTransformer replaces IR nodes before or after transforming their children.
// Enter replacements determine the children traversed; exit replacements are
// not traversed again. Callbacks receive owned copies and may edit them in place
// or return replacement values. Returned replacements are copied before use.
// Definition callback views refer to owned nodes, unlike IRVisitor's borrows.
//
// Transformations return a fresh transport graph and preserve input metadata.
// Source/analysis tables and runtime provenance are not recomputed when IDs or
// names change. Like the upstream transformer, type application constructors
// and instance override parameters are copied but not transformed. Sum fields
// must remain concrete rows, and let definitions must remain operators.
//
// The zero value and a nil transformer perform identity copies. Operations are
// synchronous; callbacks can return errors, including cancellation errors.
// A transformer and its callbacks belong to one sequential transformation.
// Unlike IRVisitor, the upstream transformer has no definition-depth state.
// Inputs and replacements must be finite, acyclic IR. Traversal checks the
// required children it follows; this is not full IR validation. In particular,
// exit replacements are not traversed, and their validity is the callback's
// responsibility. OperatorDef references represent the def variant; their Kind
// field is normalized when converting between local and top-level definitions.
type IRTransformer struct {
	EnterModule, ExitModule           func(Module) (Module, error)
	EnterDeclaration, ExitDeclaration func(Declaration) (Declaration, error)
	EnterDefinition, ExitDefinition   func(IRDefinition) (IRDefinition, error)
	EnterExpression, ExitExpression   func(Expr) (Expr, error)
	EnterType, ExitType               func(Type) (Type, error)
	EnterRow, ExitRow                 func(Row) (Row, error)
}

func irTransformApply[T any](callback func(T) (T, error), node T, clone func(T) T) (T, error) {
	if callback == nil {
		return node, nil
	}
	replacement, err := callback(node)
	if err != nil {
		var zero T
		return zero, err
	}
	return clone(replacement), nil
}

func irTransformInstance(t *IRTransformer) *IRTransformer {
	if t == nil {
		return &IRTransformer{}
	}
	return t
}

func (t *IRTransformer) TransformModule(module Module) (Module, error) {
	return irTransformInstance(t).module(irTransformCloneModule(module))
}

func (t *IRTransformer) module(module Module) (Module, error) {
	module, err := irTransformApply(t.EnterModule, module, irTransformCloneModule)
	if err != nil {
		return Module{}, err
	}
	for i := range module.Declarations {
		module.Declarations[i], err = t.declaration(module.Declarations[i])
		if err != nil {
			return Module{}, err
		}
	}
	return irTransformApply(t.ExitModule, module, irTransformCloneModule)
}

func (t *IRTransformer) TransformDeclaration(declaration Declaration) (Declaration, error) {
	return irTransformInstance(t).declaration(irTransformCloneDeclaration(declaration))
}

func (t *IRTransformer) declaration(declaration Declaration) (Declaration, error) {
	declaration, err := irTransformApply(t.EnterDeclaration, declaration, irTransformCloneDeclaration)
	if err != nil {
		return Declaration{}, err
	}
	switch declaration.Kind {
	case "def", "const", "var", "assume", "typedef":
		definition, err := t.definition(IRDefinition{Declaration: &declaration})
		if err != nil {
			return Declaration{}, err
		}
		if definition.Declaration != nil {
			declaration = *definition.Declaration
		} else {
			declaration = irTransformDeclaration(*definition.Operator)
		}
	case "instance":
		for i := range declaration.Overrides {
			declaration.Overrides[i].Expr, err = t.expression(declaration.Overrides[i].Expr)
			if err != nil {
				return Declaration{}, err
			}
		}
	case "import", "export":
	default:
		return Declaration{}, fmt.Errorf("unknown declaration kind %q", declaration.Kind)
	}
	return irTransformApply(t.ExitDeclaration, declaration, irTransformCloneDeclaration)
}

func (t *IRTransformer) TransformDefinition(definition IRDefinition) (IRDefinition, error) {
	return irTransformInstance(t).definition(irTransformCloneDefinition(definition))
}

func (t *IRTransformer) definition(definition IRDefinition) (IRDefinition, error) {
	definition, err := irTransformApply(t.EnterDefinition, definition, irTransformCloneDefinition)
	if err != nil {
		return IRDefinition{}, err
	}
	if !definition.Valid() {
		return IRDefinition{}, fmt.Errorf("IR definition must reference exactly one declaration or local operator")
	}
	var declaration Declaration
	if definition.Declaration != nil {
		declaration = *definition.Declaration
	} else {
		declaration = irTransformDeclaration(*definition.Operator)
	}
	if declaration.TypeAnnotation != nil {
		typ, err := t.typ(*declaration.TypeAnnotation)
		if err != nil {
			return IRDefinition{}, err
		}
		declaration.TypeAnnotation = &typ
	} else if declaration.Kind == "typedef" && declaration.Type != nil {
		typ, err := t.typ(*declaration.Type)
		if err != nil {
			return IRDefinition{}, err
		}
		declaration.Type = &typ
	}
	switch declaration.Kind {
	case "def", "assume":
		body := declaration.Expr
		if declaration.Kind == "assume" {
			body = declaration.Assumption
		}
		if body == nil {
			return IRDefinition{}, fmt.Errorf("%s definition missing expression", declaration.Kind)
		}
		expression, err := t.expression(*body)
		if err != nil {
			return IRDefinition{}, err
		}
		if declaration.Kind == "assume" {
			declaration.Assumption = &expression
		} else {
			declaration.Expr = &expression
		}
	case "const", "var", "typedef":
	default:
		return IRDefinition{}, fmt.Errorf("unknown definition kind %q", declaration.Kind)
	}
	if definition.Declaration != nil {
		definition.Declaration = &declaration
	} else {
		operator := irTransformOperator(declaration)
		definition.Operator = &operator
	}
	definition, err = irTransformApply(t.ExitDefinition, definition, irTransformCloneDefinition)
	if err != nil {
		return IRDefinition{}, err
	}
	if !definition.Valid() {
		return IRDefinition{}, fmt.Errorf("IR definition must reference exactly one declaration or local operator")
	}
	return definition, nil
}

func (t *IRTransformer) TransformExpression(expression Expr) (Expr, error) {
	return irTransformInstance(t).expression(irTransformCloneExpression(expression))
}

func (t *IRTransformer) expression(expression Expr) (Expr, error) {
	expression, err := irTransformApply(t.EnterExpression, expression, irTransformCloneExpression)
	if err != nil {
		return Expr{}, err
	}
	switch expression.Kind {
	case "name", "bool", "int", "str":
	case "app":
		for i := range expression.Args {
			expression.Args[i], err = t.expression(expression.Args[i])
			if err != nil {
				return Expr{}, err
			}
		}
	case "lambda", "let":
		if expression.Body == nil {
			return Expr{}, fmt.Errorf("%s expression missing body", expression.Kind)
		}
		if expression.Kind == "lambda" {
			for i := range expression.Params {
				parameter := &expression.Params[i]
				if parameter.TypeAnnotation != nil {
					typ, err := t.typ(*parameter.TypeAnnotation)
					if err != nil {
						return Expr{}, err
					}
					parameter.TypeAnnotation = &typ
				}
			}
		} else {
			definition, err := t.definition(IRDefinition{Operator: expression.Opdef})
			if err != nil {
				return Expr{}, err
			}
			if definition.Kind() != "def" {
				return Expr{}, fmt.Errorf("let operator definition transformed into non-operator definition")
			}
			if definition.Operator != nil {
				expression.Opdef = definition.Operator
			} else {
				operator := irTransformOperator(*definition.Declaration)
				expression.Opdef = &operator
			}
		}
		body, err := t.expression(*expression.Body)
		if err != nil {
			return Expr{}, err
		}
		expression.Body = &body
	default:
		return Expr{}, fmt.Errorf("unknown expression kind %q", expression.Kind)
	}
	return irTransformApply(t.ExitExpression, expression, irTransformCloneExpression)
}

func (t *IRTransformer) TransformType(typ Type) (Type, error) {
	return irTransformInstance(t).typ(irTransformCloneType(typ))
}

func (t *IRTransformer) typ(typ Type) (Type, error) {
	typ, err := irTransformApply(t.EnterType, typ, irTransformCloneType)
	if err != nil {
		return Type{}, err
	}
	switch typ.Kind {
	case "bool", "int", "str", "const", "var":
	case "set", "list":
		if typ.Elem == nil {
			return Type{}, fmt.Errorf("%s type missing element", typ.Kind)
		}
		element, err := t.typ(*typ.Elem)
		if err != nil {
			return Type{}, err
		}
		typ.Elem = &element
	case "fun":
		if typ.Arg == nil || typ.Res == nil {
			return Type{}, fmt.Errorf("function type missing argument or result")
		}
		argument, err := t.typ(*typ.Arg)
		if err != nil {
			return Type{}, err
		}
		result, err := t.typ(*typ.Res)
		if err != nil {
			return Type{}, err
		}
		typ.Arg, typ.Res = &argument, &result
	case "oper", "app":
		for i := range typ.Args {
			typ.Args[i], err = t.typ(typ.Args[i])
			if err != nil {
				return Type{}, err
			}
		}
		if typ.Kind == "oper" {
			if typ.Res == nil {
				return Type{}, fmt.Errorf("operator type missing result")
			}
			result, err := t.typ(*typ.Res)
			if err != nil {
				return Type{}, err
			}
			typ.Res = &result
		}
	case "tup", "rec", "sum":
		if typ.Fields == nil {
			return Type{}, fmt.Errorf("%s type missing row", typ.Kind)
		}
		row, err := t.row(*typ.Fields)
		if err != nil {
			return Type{}, err
		}
		if typ.Kind == "sum" && row.Kind != "row" {
			return Type{}, fmt.Errorf("sum type fields transformed into non-row")
		}
		typ.Fields = &row
	default:
		return Type{}, fmt.Errorf("unknown type kind %q", typ.Kind)
	}
	return irTransformApply(t.ExitType, typ, irTransformCloneType)
}

func (t *IRTransformer) TransformRow(row Row) (Row, error) {
	return irTransformInstance(t).row(irTransformCloneRow(row))
}

func (t *IRTransformer) row(row Row) (Row, error) {
	row, err := irTransformApply(t.EnterRow, row, irTransformCloneRow)
	if err != nil {
		return Row{}, err
	}
	switch row.Kind {
	case "empty", "var":
	case "row":
		for i := range row.Fields {
			row.Fields[i].Type, err = t.typ(row.Fields[i].Type)
			if err != nil {
				return Row{}, err
			}
		}
		if row.Other == nil {
			return Row{}, fmt.Errorf("concrete row missing tail")
		}
		other, err := t.row(*row.Other)
		if err != nil {
			return Row{}, err
		}
		row.Other = &other
	default:
		return Row{}, fmt.Errorf("unknown row kind %q", row.Kind)
	}
	return irTransformApply(t.ExitRow, row, irTransformCloneRow)
}

func irTransformPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func irTransformSlice[T any](values []T, clone func(T) T) []T {
	if values == nil {
		return nil
	}
	out := make([]T, len(values))
	for i, value := range values {
		out[i] = clone(value)
	}
	return out
}

func irTransformNodePointer[T any](value *T, clone func(T) T) *T {
	if value == nil {
		return nil
	}
	copy := clone(*value)
	return &copy
}

func irTransformCloneModule(module Module) Module {
	module.Doc = irTransformPointer(module.Doc)
	module.Declarations = irTransformSlice(module.Declarations, irTransformCloneDeclaration)
	return module
}

func irTransformCloneDeclaration(d Declaration) Declaration {
	d.runtimeVariable = irTransformPointer(d.runtimeVariable)
	d.Hidden, d.Depth, d.Doc = irTransformPointer(d.Hidden), irTransformPointer(d.Depth), irTransformPointer(d.Doc)
	d.DefName, d.QualifiedName, d.FromSource = irTransformPointer(d.DefName), irTransformPointer(d.QualifiedName), irTransformPointer(d.FromSource)
	d.Namespaces, d.Params = slices.Clone(d.Namespaces), slices.Clone(d.Params)
	d.Shadowing, d.ImportedFrom = slices.Clone(d.Shadowing), slices.Clone(d.ImportedFrom)
	d.Expr, d.Assumption = irTransformNodePointer(d.Expr, irTransformCloneExpression), irTransformNodePointer(d.Assumption, irTransformCloneExpression)
	d.TypeAnnotation, d.Type = irTransformNodePointer(d.TypeAnnotation, irTransformCloneType), irTransformNodePointer(d.Type, irTransformCloneType)
	d.Overrides = irTransformSlice(d.Overrides, func(o Override) Override {
		o.Parameter, o.Expr = irTransformCloneParameter(o.Parameter), irTransformCloneExpression(o.Expr)
		return o
	})
	return d
}

func irTransformCloneDefinition(d IRDefinition) IRDefinition {
	d.Declaration = irTransformNodePointer(d.Declaration, irTransformCloneDeclaration)
	d.Operator = irTransformNodePointer(d.Operator, func(o OperatorDef) OperatorDef {
		copy := irTransformOperator(irTransformCloneDeclaration(irTransformDeclaration(o)))
		copy.Kind = o.Kind
		return copy
	})
	return d
}

func irTransformCloneExpression(e Expr) Expr {
	e.runtimeSource = irTransformPointer(e.runtimeSource)
	e.Args = irTransformSlice(e.Args, irTransformCloneExpression)
	e.Params = irTransformSlice(e.Params, irTransformCloneParameter)
	e.Body = irTransformNodePointer(e.Body, irTransformCloneExpression)
	e.Opdef = irTransformCloneDefinition(IRDefinition{Operator: e.Opdef}).Operator
	return e
}

func irTransformCloneParameter(p Parameter) Parameter {
	p.Depth, p.Shadowing = irTransformPointer(p.Depth), slices.Clone(p.Shadowing)
	p.TypeAnnotation = irTransformNodePointer(p.TypeAnnotation, irTransformCloneType)
	return p
}

func irTransformCloneType(t Type) Type {
	t.ID = irTransformPointer(t.ID)
	t.Elem, t.Arg = irTransformNodePointer(t.Elem, irTransformCloneType), irTransformNodePointer(t.Arg, irTransformCloneType)
	t.Res, t.Ctor = irTransformNodePointer(t.Res, irTransformCloneType), irTransformNodePointer(t.Ctor, irTransformCloneType)
	t.Args = irTransformSlice(t.Args, irTransformCloneType)
	t.Fields = irTransformNodePointer(t.Fields, irTransformCloneRow)
	return t
}

func irTransformCloneRow(r Row) Row {
	r.Fields = irTransformSlice(r.Fields, func(field RowField) RowField {
		field.Type = irTransformCloneType(field.Type)
		return field
	})
	r.Other = irTransformNodePointer(r.Other, irTransformCloneRow)
	return r
}

func irTransformDeclaration(o OperatorDef) Declaration {
	return Declaration{Hidden: o.Hidden, Namespaces: o.Namespaces, Shadowing: o.Shadowing, ImportedFrom: o.ImportedFrom,
		Depth: o.Depth, ID: o.ID, Kind: "def", Name: o.Name, Qualifier: o.Qualifier, Expr: o.Expr,
		TypeAnnotation: o.TypeAnnotation, Doc: o.Doc}
}

func irTransformOperator(d Declaration) OperatorDef {
	return OperatorDef{Hidden: d.Hidden, Namespaces: d.Namespaces, Shadowing: d.Shadowing, ImportedFrom: d.ImportedFrom,
		Depth: d.Depth, ID: d.ID, Kind: "def", Name: d.Name, Qualifier: d.Qualifier, Expr: d.Expr,
		TypeAnnotation: d.TypeAnnotation, Doc: d.Doc}
}
