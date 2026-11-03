package quint

// A reference retains the source builder's memo and the instance wrappers that
// surround its getter. Constant and parameter registers themselves use ID alone.
type runtimeSourceValueTarget struct {
	Kind      string
	Name      string
	Identity  runtimeSourceIdentity
	Overrides []runtimeSourceValueOverride
}

type runtimeSourceValueOverride struct {
	ConstantID ID
	Name       string
	Value      runtimeSourceValueTarget
}

func (p *runtimeSourcePreparation) sourceValueTarget(definition LookupDefinition, compilation runtimeSourceCompileContext) (runtimeSourceValueTarget, error) {
	target := runtimeSourceValueTarget{Name: definition.Name}
	if definition.Parameter != nil {
		target.Kind, target.Name = "param", definition.Parameter.Name
	} else if definition.Declaration != nil {
		target.Kind, target.Name = definition.Declaration.Kind, definition.Declaration.Name
	}
	if instance := definition.ImportedFrom; instance != nil && instance.Kind == "instance" {
		compilation = runtimeSourceCompileContext{instance: instance.ID, namespaces: definition.Namespaces}
		for _, override := range instance.Overrides {
			constant, found := p.source.graph.table.Get2(override.Parameter.ID)
			if !found {
				id := override.Parameter.ID
				return runtimeSourceValueTarget{}, Diagnostic{Code: "QNT501", Message: "Runtime source override constant not found: " + override.Parameter.Name, Reference: &id}
			}
			target.Overrides = append(target.Overrides, runtimeSourceValueOverride{
				ConstantID: constant.ID, Name: override.Parameter.Name,
				Value: runtimeSourceValueTarget{Kind: "def", Name: override.Parameter.Name, Identity: runtimeSourceIdentity{ID: override.Parameter.ID, InstanceID: compilation.instance}},
			})
		}
	}
	target.Identity = runtimeSourceIdentity{ID: definition.ID, InstanceID: compilation.instance}
	return target, nil
}

func (p *runtimeSourcePreparation) captureValueReference(identity runtimeSourceIdentity, definition LookupDefinition, compilation runtimeSourceCompileContext) error {
	target, err := p.sourceValueTarget(definition, compilation)
	if err == nil {
		p.values.Set(identity, target)
	}
	return err
}

func (p *runtimeSourcePreparation) captureSourceDefinition(definition LookupDefinition, compilation runtimeSourceCompileContext) {
	declaration := definition.Declaration
	if declaration == nil || declaration.Kind != "def" || declaration.Expr == nil {
		return
	}
	identity := runtimeSourceIdentity{ID: definition.ID, InstanceID: compilation.instance}
	if _, exists := p.definitions.Get2(identity); exists {
		return
	}
	body := sourceRuntimeExpression(*declaration.Expr, compilation.instance)
	depth := definition.Depth
	p.definitions.Set(identity, OperatorDef{Kind: "def", ID: definition.ID, Name: declaration.Name, Qualifier: declaration.Qualifier, Expr: &body, Depth: &depth})
}

func (p *runtimeSourcePreparation) ValueReference(expression Expr) (runtimeSourceValueTarget, bool) {
	if p == nil || expression.runtimeSource == nil {
		return runtimeSourceValueTarget{}, false
	}
	return p.values.Get2(*expression.runtimeSource)
}

func (p *runtimeSourcePreparation) DefinitionIdentity(definition OperatorDef) (runtimeSourceIdentity, bool) {
	if p == nil || definition.Expr == nil || definition.Expr.runtimeSource == nil {
		return runtimeSourceIdentity{}, false
	}
	identity := *definition.Expr.runtimeSource
	owner, exists := p.owners.Get2(identity.ID.String())
	if !exists {
		return runtimeSourceIdentity{}, false
	}
	identity.ID = owner
	return identity, true
}

func (p *runtimeSourcePreparation) ExpressionIdentity(expression Expr) (runtimeSourceIdentity, bool) {
	if p == nil || expression.runtimeSource == nil {
		return runtimeSourceIdentity{}, false
	}
	return *expression.runtimeSource, true
}

func (p *runtimeSourcePreparation) ParameterIDs(lambda Expr) ([]ID, bool) {
	if p == nil || lambda.runtimeSource == nil {
		return nil, false
	}
	return p.parameters.Get2(lambda.runtimeSource.ID.String())
}

func (p *runtimeSourcePreparation) RootIdentity(name string) (runtimeSourceIdentity, bool) {
	if p == nil {
		return runtimeSourceIdentity{}, false
	}
	return p.roots.Get2(name)
}

func (p *runtimeSourcePreparation) SourceDefinition(identity runtimeSourceIdentity) (OperatorDef, bool) {
	if p == nil {
		return OperatorDef{}, false
	}
	return p.definitions.Get2(identity)
}
