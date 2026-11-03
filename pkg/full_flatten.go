package quint

import (
	"fmt"

	"github.com/glycerine/insdict"
)

// FlatteningResult retains every flattened module in dependency order and the
// lookup and analysis tables needed by later compilation stages.
type FlatteningResult struct {
	Modules  []Module
	Table    *insdict.Dict[ID, LookupDefinition]
	Analysis AnalysisOutput
}

// FlattenModules unshadows names, inlines aliases, expands imports/exports and
// instances, and resolves names after each module. ids must
// continue the generator used for parsing. Like upstream, this pass extends
// sources and analysis metadata when creating instance definitions.
func FlattenModules(modules []Module, table *insdict.Dict[ID, LookupDefinition], ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) (FlatteningResult, error) {
	var sourceGraph *runtimeSourceGraph
	// Cell bindings and synthetic action frames need original source identity.
	// Other pure projects avoid an extra graph copy and name-resolution pass.
	needsSourceGraph := false
	for _, module := range modules {
		for _, declaration := range module.Declarations {
			if declaration.Kind == "var" || declaration.Expr != nil && (declaration.Qualifier == Action && declaration.Expr.Kind != "lambda" || runtimeHasSourceAction(*declaration.Expr)) {
				needsSourceGraph = true
			}
		}
		if sourceGraph == nil && module.runtimeSource != nil {
			sourceGraph = module.runtimeSource.graph
		}
	}
	if sourceGraph == nil && needsSourceGraph {
		var err error
		sourceGraph, err = newRuntimeSourceGraph(modules)
		if err != nil {
			return FlatteningResult{}, err
		}
	}
	unique := make([]Module, len(modules))
	workingTable := NewExpressionNameResolver(nil).Table
	for id, def := range table.All() {
		if def.Declaration != nil {
			declaration := seedRuntimeVariableProvenance(*def.Declaration)
			if sourceGraph != nil {
				declaration = sourceRuntimeDeclaration(declaration, ID{})
			}
			def.Declaration = &declaration
		}
		workingTable.Set(id, def)
	}
	for i, m := range modules {
		if m.runtimeSource == nil && sourceGraph != nil {
			m.runtimeSource = &runtimeSourceModule{graph: sourceGraph, name: m.Name}
		}
		m.Declarations = append([]Declaration{}, m.Declarations...)
		for j, declaration := range m.Declarations {
			declaration = seedRuntimeVariableProvenance(declaration)
			if sourceGraph != nil {
				declaration = sourceRuntimeDeclaration(declaration, ID{})
			}
			m.Declarations[j] = declaration
		}
		unique[i] = UnshadowNames(m, workingTable)
	}
	inlined, err := InlineTypeAliases(unique, workingTable, analysis)
	if err != nil {
		return FlatteningResult{}, err
	}
	registry := insdict.NewDict[string, Module]()
	for _, m := range inlined.Modules {
		registry.Set(m.Name, m)
	}
	result := FlatteningResult{Modules: []Module{}, Table: inlined.Table, Analysis: inlined.Analysis}
	queue := inlined.Modules
	resolve := func(ms []Module) ([]Module, *insdict.Dict[ID, LookupDefinition], error) {
		linked, err := ResolveLinkedModules(ms)
		if err != nil {
			return nil, nil, err
		}
		if len(linked.Errors) > 0 {
			return nil, nil, fmt.Errorf("resolving flattened modules: %v", linked.Errors)
		}
		return ms, linked.Table, nil
	}
	for len(queue) > 0 {
		module := queue[0]
		queue = queue[1:]
		first, err := FlattenModule(module, registry, result.Table)
		if err != nil {
			return FlatteningResult{}, err
		}
		instances, err := FlattenInstances(first, registry, result.Table, ids, sources, result.Analysis)
		if err != nil {
			return FlatteningResult{}, err
		}
		_, intermediate, err := resolve(append(append([]Module{}, result.Modules...), instances...))
		if err != nil {
			return FlatteningResult{}, err
		}
		for _, m := range instances {
			flat, err := FlattenModule(m, registry, intermediate)
			if err != nil {
				return FlatteningResult{}, err
			}
			result.Modules = append(result.Modules, flat)
			registry.Set(m.Name, flat)
		}
		all, updated, err := resolve(append(append([]Module{}, result.Modules...), queue...))
		if err != nil {
			return FlatteningResult{}, err
		}
		result.Modules = all[:len(result.Modules)]
		result.Table = updated
	}
	for _, m := range result.Modules {
		for _, d := range m.Declarations {
			if d.Kind == "import" || d.Kind == "export" || d.Kind == "instance" {
				return FlatteningResult{}, fmt.Errorf("unexpanded %s in flattened module %s", d.Kind, m.Name)
			}
		}
	}
	return result, nil
}
