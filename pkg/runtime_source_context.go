package quint

import (
	"context"
	"hash/fnv"
	"strings"

	"github.com/glycerine/insdict"
)

// Source IDs are stable across flattening. An instance owns a separate compiler
// memo, while namespaces can change within that memo through exported aliases.
type runtimeSourceIdentity struct {
	ID, InstanceID ID
}

type runtimeSourceGraph struct {
	table   *insdict.Dict[ID, LookupDefinition]
	scopes  *insdict.Dict[string, *NameScope]
	actions *insdict.Dict[string, runtimeSourceAction]
}

type runtimeSourceAction struct {
	ID   ID
	Name string
}

type runtimeSourceModule struct {
	graph *runtimeSourceGraph
	name  string
}

func newRuntimeSourceGraph(modules []Module) (*runtimeSourceGraph, error) {
	owned := make([]Module, len(modules))
	for i, module := range modules {
		owned[i] = module
		owned[i].Declarations = make([]Declaration, len(module.Declarations))
		for j, declaration := range module.Declarations {
			copy := declaration
			if declaration.Expr != nil {
				expression := copyRuntimeExpression(*declaration.Expr)
				copy.Expr = &expression
			}
			if declaration.Assumption != nil {
				expression := copyRuntimeExpression(*declaration.Assumption)
				copy.Assumption = &expression
			}
			copy.Overrides = append([]Override{}, declaration.Overrides...)
			for k := range copy.Overrides {
				copy.Overrides[k].Expr = copyRuntimeExpression(copy.Overrides[k].Expr)
			}
			owned[i].Declarations[j] = copy
		}
	}
	linked, err := ResolveLinkedModules(owned)
	if err != nil {
		return nil, err
	}
	if len(linked.Errors) != 0 {
		return nil, linked.Errors[0]
	}
	actions := insdict.NewDict[string, runtimeSourceAction]()
	for id, definition := range linked.Table.All() {
		declaration := definition.Declaration
		if declaration != nil && declaration.ID == id && declaration.Kind == "def" && declaration.Qualifier == Action && declaration.Expr != nil {
			actions.Set(declaration.Expr.ID.String(), runtimeSourceAction{ID: declaration.ID, Name: declaration.Name})
		}
	}
	return &runtimeSourceGraph{table: linked.Table, scopes: linked.Modules, actions: actions}, nil
}

// Nonlambda actions create synthetic recorder calls even without variables.
// Their original declaration names must survive namespacing and unshadowing.
func runtimeHasSourceAction(expression Expr) bool {
	if expression.Opdef != nil && expression.Opdef.Expr != nil {
		if expression.Opdef.Qualifier == Action && expression.Opdef.Expr.Kind != "lambda" {
			return true
		}
		if runtimeHasSourceAction(*expression.Opdef.Expr) {
			return true
		}
	}
	for _, argument := range expression.Args {
		if runtimeHasSourceAction(argument) {
			return true
		}
	}
	return expression.Body != nil && runtimeHasSourceAction(*expression.Body)
}

type runtimeSourceNode struct {
	variable *RuntimeVariableBinding
}

type runtimeSourceCompileContext struct {
	instance   ID
	namespaces []string
}

// runtimeSourcePreparation retains the reference builder's compilation history.
// It registers variables without evaluating expressions or consuming randomness.
// Immutable per-reference bindings are consumed by the existing evaluator; the
// flattened name remains a fallback for transported IR without source metadata.
// Like a builder, this object belongs to one sequential evaluator and storage.
type runtimeSourcePreparation struct {
	source   *runtimeSourceModule
	memos    *insdict.Dict[ID, *insdict.Dict[ID, *runtimeSourceNode]]
	bindings *insdict.Dict[runtimeSourceIdentity, RuntimeVariableBinding]
	active   map[runtimeSourceIdentity]bool
}

func runtimeSourceIDHash(id ID) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(id.String()))
	return hash.Sum64()
}

func runtimeSourceIdentityHash(identity runtimeSourceIdentity) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(identity.ID.String()))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(identity.InstanceID.String()))
	return hash.Sum64()
}

func newRuntimeSourcePreparation(source *runtimeSourceModule) *runtimeSourcePreparation {
	return &runtimeSourcePreparation{
		source: source, memos: insdict.NewDictFunc[ID, *insdict.Dict[ID, *runtimeSourceNode]](runtimeSourceIDHash),
		bindings: insdict.NewDictFunc[runtimeSourceIdentity, RuntimeVariableBinding](runtimeSourceIdentityHash), active: map[runtimeSourceIdentity]bool{},
	}
}

// UpdateSource changes the lookup graph used for future compilations. Memoized
// references and their captured bindings remain frozen across REPL requests.
func (p *runtimeSourcePreparation) UpdateSource(source *runtimeSourceModule) {
	p.source = source
}

func (p *runtimeSourcePreparation) Binding(identity runtimeSourceIdentity) (RuntimeVariableBinding, bool) {
	if p == nil {
		return RuntimeVariableBinding{}, false
	}
	return p.bindings.Get2(identity)
}

// ActionApp freezes the original source declaration in a compiled getter.
// Transported IR has no private source identity and retains its flattened name.
func (p *runtimeSourcePreparation) ActionApp(definition OperatorDef) Expr {
	app := Expr{ID: definition.ID, Kind: "app", Opcode: definition.Name, Args: []Expr{}}
	if p == nil || p.source == nil || p.source.graph == nil || p.source.graph.actions == nil || definition.Expr == nil || definition.Expr.runtimeSource == nil {
		return app
	}
	if source, exists := p.source.graph.actions.Get2(definition.Expr.runtimeSource.ID.String()); exists {
		app.ID, app.Opcode = source.ID, source.Name
	}
	return app
}

func (p *runtimeSourcePreparation) PrepareDefinition(ctx context.Context, name string, storage *VariableStorage) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if p == nil || p.source == nil || p.source.graph == nil || storage == nil {
		return false, nil
	}
	scope, exists := p.source.graph.scopes.Get2(p.source.name)
	if !exists {
		return false, nil
	}
	definition, exists := scope.Lookup(name)
	if !exists {
		definition, exists = scope.Lookup(strings.TrimPrefix(name, p.source.name+"::"))
	}
	if !exists {
		return false, nil
	}
	_, err := p.buildDefinition(ctx, definition, runtimeSourceCompileContext{}, storage)
	return true, err
}

func (p *runtimeSourcePreparation) memo(instance ID) *insdict.Dict[ID, *runtimeSourceNode] {
	memo, exists := p.memos.Get2(instance)
	if !exists {
		memo = insdict.NewDictFunc[ID, *runtimeSourceNode](runtimeSourceIDHash)
		p.memos.Set(instance, memo)
	}
	return memo
}

func sourceVariableBinding(definition LookupDefinition, compilation runtimeSourceCompileContext) RuntimeVariableBinding {
	key := definition.ID.String()
	for _, namespace := range compilation.namespaces {
		key += "#" + namespace
	}
	name := definition.Name
	for _, namespace := range compilation.namespaces {
		name = namespace + "::" + name
	}
	return RuntimeVariableBinding{Key: key, Name: name}
}

func (p *runtimeSourcePreparation) underDefinition(ctx context.Context, definition LookupDefinition, compilation runtimeSourceCompileContext, storage *VariableStorage, build func(runtimeSourceCompileContext) (*runtimeSourceNode, error)) (*runtimeSourceNode, error) {
	instance := definition.ImportedFrom
	if instance == nil || instance.Kind != "instance" {
		return build(compilation)
	}
	compilation = runtimeSourceCompileContext{instance: instance.ID, namespaces: definition.Namespaces}
	p.memo(compilation.instance)
	// Overrides are compiled under the entered instance even on a memo hit.
	for _, override := range instance.Overrides {
		depth := 0
		declaration := Declaration{Kind: "def", ID: override.Parameter.ID, Name: override.Parameter.Name, Qualifier: PureVal, Expr: &override.Expr, Depth: &depth}
		if _, err := p.buildDefinition(ctx, LookupDefinition{ID: declaration.ID, Name: declaration.Name, Declaration: &declaration}, compilation, storage); err != nil {
			return nil, err
		}
	}
	return build(compilation)
}

func (p *runtimeSourcePreparation) buildDefinition(ctx context.Context, definition LookupDefinition, compilation runtimeSourceCompileContext, storage *VariableStorage) (*runtimeSourceNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.underDefinition(ctx, definition, compilation, storage, func(compilation runtimeSourceCompileContext) (*runtimeSourceNode, error) {
		memo := p.memo(compilation.instance)
		if node, exists := memo.Get2(definition.ID); exists {
			return node, nil
		}
		identity := runtimeSourceIdentity{ID: definition.ID, InstanceID: compilation.instance}
		if p.active[identity] {
			return nil, Diagnostic{Code: "QNT501", Message: "Cyclic runtime source compilation", Reference: &identity.ID}
		}
		p.active[identity] = true
		defer delete(p.active, identity)
		node := &runtimeSourceNode{}
		if definition.Declaration != nil {
			switch definition.Declaration.Kind {
			case "var":
				binding := sourceVariableBinding(definition, compilation)
				storage.Register(binding.Key, binding.Name)
				node.variable = &binding
			case "def":
				if definition.Declaration.Expr != nil {
					var err error
					node, err = p.buildExpression(ctx, *definition.Declaration.Expr, compilation, storage)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		memo.Set(definition.ID, node)
		return node, nil
	})
}

func (p *runtimeSourcePreparation) buildExpression(ctx context.Context, expression Expr, compilation runtimeSourceCompileContext, storage *VariableStorage) (*runtimeSourceNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	memo := p.memo(compilation.instance)
	if node, exists := memo.Get2(expression.ID); exists {
		return node, nil
	}
	identity := runtimeSourceIdentity{ID: expression.ID, InstanceID: compilation.instance}
	if p.active[identity] {
		return nil, Diagnostic{Code: "QNT501", Message: "Cyclic runtime source compilation", Reference: &identity.ID}
	}
	p.active[identity] = true
	defer delete(p.active, identity)
	node := &runtimeSourceNode{}
	build := func(expression *Expr) error {
		if expression == nil {
			return nil
		}
		_, err := p.buildExpression(ctx, *expression, compilation, storage)
		return err
	}
	switch expression.Kind {
	case "name":
		if definition, exists := p.source.graph.table.Get2(expression.ID); exists {
			var err error
			node, err = p.buildDefinition(ctx, definition, compilation, storage)
			if err != nil {
				return nil, err
			}
			if definition.Declaration != nil && definition.Declaration.Kind == "var" && node.variable != nil {
				p.bindings.Set(identity, *node.variable)
			}
		}
	case "lambda":
		if err := build(expression.Body); err != nil {
			return nil, err
		}
	case "let":
		if expression.Opdef != nil && expression.Opdef.Qualifier == Nondet {
			// The let installs its sampled-value getter before building the set.
			memo.Set(expression.Opdef.ID, &runtimeSourceNode{})
			if expression.Opdef.Expr != nil && len(expression.Opdef.Expr.Args) > 0 {
				if err := build(&expression.Opdef.Expr.Args[0]); err != nil {
					return nil, err
				}
			}
		}
		if err := build(expression.Body); err != nil {
			return nil, err
		}
	case "app":
		if expression.Opcode == "assign" && len(expression.Args) == 2 {
			if err := build(&expression.Args[1]); err != nil {
				return nil, err
			}
			definition, exists := p.source.graph.table.Get2(expression.Args[0].ID)
			if exists {
				_, err := p.underDefinition(ctx, definition, compilation, storage, func(targetContext runtimeSourceCompileContext) (*runtimeSourceNode, error) {
					binding := sourceVariableBinding(definition, targetContext)
					storage.Register(binding.Key, binding.Name)
					p.bindings.Set(runtimeSourceIdentity{ID: expression.Args[0].ID, InstanceID: compilation.instance}, binding)
					return &runtimeSourceNode{variable: &binding}, nil
				})
				if err != nil {
					return nil, err
				}
			}
		} else {
			for i := range expression.Args {
				if err := build(&expression.Args[i]); err != nil {
					return nil, err
				}
			}
			if definition, exists := p.source.graph.table.Get2(expression.ID); exists {
				if _, err := p.buildDefinition(ctx, definition, compilation, storage); err != nil {
					return nil, err
				}
			}
		}
	}
	memo.Set(expression.ID, node)
	return node, nil
}
