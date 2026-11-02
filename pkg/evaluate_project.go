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
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if ids == nil {
		return RuntimeValue{}, fmt.Errorf("pure project evaluation requires the parser ID generator")
	}
	if _, err := SelectMainModule(modules, main, "", ""); err != nil {
		return RuntimeValue{}, err
	}
	sorted, diagnostics := SortModules(modules)
	if len(diagnostics) > 0 {
		return RuntimeValue{}, diagnostics[0]
	}
	linked, err := ResolveLinkedModules(sorted)
	if err != nil {
		return RuntimeValue{}, err
	}
	if len(linked.Errors) > 0 {
		return RuntimeValue{}, linked.Errors[0]
	}
	sorted, diagnostics = SortModuleDeclarations(sorted, linked.Table)
	if len(diagnostics) > 0 {
		return RuntimeValue{}, diagnostics[0]
	}
	analysis, err := AnalyzeModules(sorted, linked.Table)
	if err != nil {
		return RuntimeValue{}, err
	}
	if len(analysis.Diagnostics) > 0 {
		return RuntimeValue{}, analysis.Diagnostics[0]
	}
	privateIDs := NewIDGenerator()
	privateIDs.next.Set(&ids.next)
	flat, err := FlattenModules(analysis.Modules, linked.Table, privateIDs, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
	if err != nil {
		return RuntimeValue{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	for _, module := range flat.Modules {
		if module.Name == main {
			return EvaluatePureDefinition(ctx, module, name, arguments)
		}
	}
	return RuntimeValue{}, fmt.Errorf("flattened module %s does not exist", main)
}
