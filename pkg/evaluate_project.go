package quint

import (
	"context"
	"fmt"
)

// EvaluatePureModules resolves, analyzes, and flattens a parsed module graph
// before evaluating a definition in main. ids must continue the parser sequence;
// a private copy is used so evaluation does not consume the caller's IDs. Static
// diagnostics stop evaluation. Stateful simulation is outside this API.
func EvaluatePureModules(ctx context.Context, modules []Module, ids *IDGenerator, main, name string, arguments []RuntimeValue) (RuntimeValue, error) {
	module, err := prepareEvaluationProject(ctx, modules, ids, main)
	if err != nil {
		return RuntimeValue{}, err
	}
	return EvaluatePureDefinition(ctx, module, name, arguments)
}

// NewProjectEvaluator prepares a checked, flattened stateful project and retains
// its module storage/RNG for subsequent named evaluations. Caller modules and IDs
// are preserved. Source loading and simulation orchestration remain caller-owned.
func NewProjectEvaluator(ctx context.Context, modules []Module, ids *IDGenerator, main string, rng *RNG, metadata bool) (*ModuleEvaluator, error) {
	module, err := prepareEvaluationProject(ctx, modules, ids, main)
	if err != nil {
		return nil, err
	}
	return NewModuleEvaluator(module, rng, metadata)
}

func prepareEvaluationProject(ctx context.Context, modules []Module, ids *IDGenerator, main string) (Module, error) {
	if err := ctx.Err(); err != nil {
		return Module{}, err
	}
	if ids == nil {
		return Module{}, fmt.Errorf("project evaluation requires the parser ID generator")
	}
	if _, err := SelectMainModule(modules, main, "", ""); err != nil {
		return Module{}, err
	}
	sorted, diagnostics := SortModules(modules)
	if len(diagnostics) > 0 {
		return Module{}, diagnostics[0]
	}
	linked, err := ResolveLinkedModules(sorted)
	if err != nil {
		return Module{}, err
	}
	if len(linked.Errors) > 0 {
		return Module{}, linked.Errors[0]
	}
	sorted, diagnostics = SortModuleDeclarations(sorted, linked.Table)
	if len(diagnostics) > 0 {
		return Module{}, diagnostics[0]
	}
	analysis, err := AnalyzeModules(sorted, linked.Table)
	if err != nil {
		return Module{}, err
	}
	if len(analysis.Diagnostics) > 0 {
		return Module{}, analysis.Diagnostics[0]
	}
	privateIDs := NewIDGenerator()
	privateIDs.next.Set(&ids.next)
	flat, err := FlattenModules(analysis.Modules, linked.Table, privateIDs, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
	if err != nil {
		return Module{}, err
	}
	if err := ctx.Err(); err != nil {
		return Module{}, err
	}
	for _, module := range flat.Modules {
		if module.Name == main {
			return module, nil
		}
	}
	return Module{}, fmt.Errorf("flattened module %s does not exist", main)
}
