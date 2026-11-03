package quint

func sourceRuntimeExpression(expression Expr, instance ID) Expr {
	out := expression
	identity := runtimeSourceIdentity{ID: expression.ID}
	if expression.runtimeSource != nil {
		identity = *expression.runtimeSource
	}
	if identity.InstanceID == (ID{}) {
		identity.InstanceID = instance
	}
	out.runtimeSource = &identity
	if expression.Args != nil {
		out.Args = make([]Expr, len(expression.Args))
		for i := range expression.Args {
			out.Args[i] = sourceRuntimeExpression(expression.Args[i], instance)
		}
	}
	if expression.Body != nil {
		body := sourceRuntimeExpression(*expression.Body, instance)
		out.Body = &body
	}
	if expression.Opdef != nil {
		definition := *expression.Opdef
		if expression.Opdef.Expr != nil {
			body := sourceRuntimeExpression(*expression.Opdef.Expr, instance)
			definition.Expr = &body
		}
		out.Opdef = &definition
	}
	return out
}

func sourceRuntimeDeclaration(declaration Declaration, instance ID) Declaration {
	out := declaration
	if declaration.Expr != nil {
		expression := sourceRuntimeExpression(*declaration.Expr, instance)
		out.Expr = &expression
	}
	if declaration.Assumption != nil {
		expression := sourceRuntimeExpression(*declaration.Assumption, instance)
		out.Assumption = &expression
	}
	if declaration.Overrides != nil {
		out.Overrides = append([]Override{}, declaration.Overrides...)
		for i := range out.Overrides {
			out.Overrides[i].Expr = sourceRuntimeExpression(out.Overrides[i].Expr, instance)
		}
	}
	return out
}
