package quint

import (
	"context"
	"fmt"
)

// NewProjectEvaluatorFromAnalysis prepares a checked project with additional
// declarations in main, reusing completed source analysis. Analysis diagnostics
// stop evaluation. Concrete additions use incremental inference; generic source
// scope, pending assumptions, and polymorphic additions retain full inference.
// Caller modules, analysis tables, additions, and the continuing parser ID generator
// remain owned by the caller. Runtime storage and RNG follow
// NewProjectEvaluator's contracts.
func NewProjectEvaluatorFromAnalysis(ctx context.Context, previous AnalysisResult, additions []Declaration, ids *IDGenerator, main string, rng *RNG, metadata bool) (*ModuleEvaluator, error) {
	flat, err := prepareAnalyzedEvaluationProject(ctx, previous, additions, ids, main)
	if err != nil {
		return nil, err
	}
	for _, module := range flat.Modules {
		if module.Name == main {
			return NewModuleEvaluator(module, rng, metadata)
		}
	}
	return nil, fmt.Errorf("flattened module %s does not exist", main)
}

func prepareAnalyzedEvaluationProject(ctx context.Context, previous AnalysisResult, additions []Declaration, ids *IDGenerator, main string) (FlatteningResult, error) {
	if err := ctx.Err(); err != nil {
		return FlatteningResult{}, err
	}
	if ids == nil {
		return FlatteningResult{}, fmt.Errorf("project evaluation requires the parser ID generator")
	}
	index, err := SelectMainModule(previous.Modules, main, "", "")
	if err != nil {
		return FlatteningResult{}, err
	}
	if len(previous.Diagnostics) > 0 {
		return FlatteningResult{}, previous.Diagnostics[0]
	}
	if previous.Types == nil || previous.Effects == nil || previous.Modes == nil {
		return FlatteningResult{}, fmt.Errorf("project evaluation requires completed source analysis")
	}
	working := append([]Module{}, previous.Modules...)
	working[index].Declarations = append(append([]Declaration{}, previous.Modules[index].Declarations...), additions...)
	working, diagnostics := SortModules(working)
	if len(diagnostics) > 0 {
		return FlatteningResult{}, diagnostics[0]
	}
	linked, err := ResolveLinkedModules(working)
	if err != nil {
		return FlatteningResult{}, err
	}
	if len(linked.Errors) > 0 {
		return FlatteningResult{}, linked.Errors[0]
	}
	working, diagnostics = SortModuleDeclarations(working, linked.Table)
	if len(diagnostics) > 0 {
		return FlatteningResult{}, diagnostics[0]
	}
	added := make(map[ID]bool, len(additions))
	for _, declaration := range additions {
		added[declaration.ID] = true
	}
	ordered := make([]Declaration, 0, len(additions))
	mainIndex := 0
	for i := range working {
		if working[i].Name != main {
			continue
		}
		mainIndex = i
		for _, declaration := range working[i].Declarations {
			if added[declaration.ID] {
				ordered = append(ordered, declaration)
			}
		}
		break
	}
	full := analyzedProjectNeedsSourceInference(previous)
	var analysis AnalysisResult
	if !full {
		analysis, err = AnalyzeIncremental(AnalysisOutput{Types: previous.Types.Clone(), Effects: previous.Effects, Modes: previous.Modes.Clone()}, linked.Table, ordered)
		// Diagnose failing additions in the original inference context, retaining
		// its first error and raw diagnostic payload, including fresh names.
		full = err != nil || len(analysis.Diagnostics) > 0
		if !full {
			// New polymorphic operators must retain the full pass's fresh-name
			// spelling as well as its typing. Ordinary boolean run selectors
			// are grounded and do not need this compatibility fallback.
			for _, declaration := range ordered {
				if scheme, found := analysis.Types.Get2(declaration.ID.String()); found {
					types, rows, typeErr := TypeNames(&scheme.Type)
					full = typeErr != nil || len(types) > 0 || len(rows) > 0
				}
				if scheme, found := analysis.Effects.Get2(declaration.ID.String()); found {
					full = full || len(scheme.EffectVariables) > 0 || len(scheme.EntityVariables) > 0
				}
				if full {
					break
				}
			}
		}
	}
	if full {
		analysis, err = AnalyzeModules(working, linked.Table)
	}
	if err != nil {
		return FlatteningResult{}, err
	}
	if len(analysis.Diagnostics) > 0 {
		return FlatteningResult{}, analysis.Diagnostics[0]
	}
	// Incremental inference also expands type applications in new annotations.
	// Keep those resolved declarations in the graph passed to flattening, rather
	// than restoring the caller's original annotations after analyzing them.
	if full {
		working = analysis.Modules
	} else {
		resolved := analysis.Modules[0].Declarations
		next := 0
		for j, declaration := range working[mainIndex].Declarations {
			if next < len(resolved) && declaration.ID == resolved[next].ID {
				working[mainIndex].Declarations[j] = resolved[next]
				next++
			}
		}
	}
	privateIDs := NewIDGenerator()
	privateIDs.next.Set(&ids.next)
	flat, err := FlattenModules(working, linked.Table, privateIDs, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
	if err != nil {
		return FlatteningResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return FlatteningResult{}, err
	}
	return flat, nil
}

// Incremental inference keeps saved schemes but intentionally starts with empty
// active scope and constraints. Recheck sources whose retained const/var types
// still have free variables, or whose assumptions may leave constraints pending.
func analyzedProjectNeedsSourceInference(previous AnalysisResult) bool {
	for _, module := range previous.Modules {
		for _, declaration := range module.Declarations {
			if declaration.Kind == "assume" {
				return true
			}
			if declaration.Kind != "const" && declaration.Kind != "var" {
				continue
			}
			scheme, found := previous.Types.Get2(declaration.ID.String())
			if !found {
				return true
			}
			types, rows, err := FreeSchemeVariables(scheme)
			if err != nil || len(types) > 0 || len(rows) > 0 {
				return true
			}
		}
	}
	return false
}
