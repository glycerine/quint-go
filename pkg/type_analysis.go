package quint

import "github.com/glycerine/insdict"

// TypeAnalysisResult is the type-only portion of analysis. Effects, modes, and
// other static checks are separate passes and are not implied by this result.
type TypeAnalysisResult struct {
	Modules           []Module
	Types             *insdict.Dict[string, TypeScheme]
	TypeErrors        *insdict.Dict[string, ErrorTree]
	ApplicationErrors *insdict.Dict[string, ErrorTree]
}

// AnalyzeModuleTypes expands applications in lookup entries, then expands and
// infers each dependency-ordered module. The lookup table is updated in place;
// source modules are preserved. Like upstream inference, pending assumptions
// are solved by later definitions rather than unconditionally at batch end.
func AnalyzeModuleTypes(modules []Module, table *insdict.Dict[ID, LookupDefinition]) (TypeAnalysisResult, error) {
	applications := NewTypeApplicationResolver(table)
	inferrer := NewTypeInferrer(table, nil)
	result := TypeAnalysisResult{Modules: append([]Module{}, modules...), Types: inferrer.Context.Types, TypeErrors: inferrer.Context.Errors, ApplicationErrors: applications.Errors}
	if err := applications.ResolveLookupTable(); err != nil {
		return result, err
	}
	for i, m := range modules {
		declarations, err := applications.ResolveDeclarations(m.Declarations)
		if err != nil {
			return result, err
		}
		result.Modules[i].Declarations = declarations
		if _, err = inferrer.InferTypes(declarations); err != nil {
			return result, err
		}
	}
	return result, nil
}
