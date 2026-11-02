package quint

import "github.com/glycerine/insdict"

// AnalysisResult retains the individual analysis tables as well as ordered
// static diagnostics. Type/effect error trees remain available to reporters.
type AnalysisResult struct {
	TypeAnalysisResult
	Effects      *insdict.Dict[string, EffectScheme]
	EffectErrors *insdict.Dict[string, ErrorTree]
	Modes        *insdict.Dict[string, Qualifier]
	Diagnostics  []Diagnostic
}

// AnalyzeModules runs the frontend analysis passes on dependency-ordered,
// name-resolved modules. Type application expansion updates the lookup table.
func AnalyzeModules(modules []Module, table *insdict.Dict[ID, LookupDefinition]) (AnalysisResult, error) {
	return analyzeModulesWithPrevious(modules, table, nil)
}

// AnalysisOutput is the retained state used to resume analysis.
type AnalysisOutput struct {
	Types   *insdict.Dict[string, TypeScheme]
	Effects *insdict.Dict[string, EffectScheme]
	Modes   *insdict.Dict[string, Qualifier]
}

// AnalyzeIncremental analyzes a new declaration batch using previous results.
// As upstream does, types and mode suggestions are reused, while the effects
// dictionary is copied before new entries are added.
func AnalyzeIncremental(previous AnalysisOutput, table *insdict.Dict[ID, LookupDefinition], declarations []Declaration) (AnalysisResult, error) {
	return analyzeModulesWithPrevious([]Module{{Declarations: declarations}}, table, &previous)
}

func analyzeModulesWithPrevious(modules []Module, table *insdict.Dict[ID, LookupDefinition], previous *AnalysisOutput) (AnalysisResult, error) {
	applications := NewTypeApplicationResolver(table)
	var seedTypes *insdict.Dict[string, TypeScheme]
	if previous != nil {
		seedTypes = previous.Types
	}
	inferrer := NewTypeInferrer(table, seedTypes)
	types := TypeAnalysisResult{Modules: append([]Module{}, modules...), Types: inferrer.Context.Types, TypeErrors: inferrer.Context.Errors, ApplicationErrors: applications.Errors}
	effects := NewEffectInferrer(table)
	modes := NewModeChecker()
	if previous != nil {
		if previous.Effects != nil {
			for id, s := range previous.Effects.All() {
				effects.Effects.Set(id, s)
			}
		}
		if previous.Modes != nil {
			modes.Suggestions = previous.Modes
		}
	}
	updates := MultipleUpdatesChecker{}
	nondet := NewNondetChecker(table)
	result := AnalysisResult{TypeAnalysisResult: types, Effects: effects.Effects, EffectErrors: effects.Errors, Modes: modes.Suggestions, Diagnostics: []Diagnostic{}}
	if err := applications.ResolveLookupTable(); err != nil {
		return result, err
	}
	for index, m := range modules {
		declarations, err := applications.ResolveDeclarations(m.Declarations)
		if err != nil {
			return result, err
		}
		result.Modules[index].Declarations = declarations
		m.Declarations = declarations
		if _, err = inferrer.InferTypes(declarations); err != nil {
			return result, err
		}
		effects.InferEffects(m.Declarations)
		schemes := []EffectScheme{}
		for _, s := range effects.Effects.All() {
			schemes = append(schemes, s)
		}
		updateErrors := updates.Check(schemes)
		nondetErrors, err := nondet.Check(types.Types, m.Declarations)
		if err != nil {
			return result, err
		}
		if err = modes.CheckModes(m.Declarations, effects.Effects); err != nil {
			return result, err
		}
		for _, trees := range []*insdict.Dict[string, ErrorTree]{types.TypeErrors, effects.Errors, applications.Errors} {
			for key, tree := range trees.All() {
				id, err := ParseID(key)
				if err != nil {
					return result, err
				}
				trace := tree
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "QNT000", Message: tree.String(), Reference: &id, Data: &DiagnosticData{Trace: &trace}})
			}
		}
		for _, d := range modes.Errors.All() {
			result.Diagnostics = append(result.Diagnostics, d)
		}
		for _, d := range updateErrors.All() {
			result.Diagnostics = append(result.Diagnostics, d)
		}
		result.Diagnostics = append(result.Diagnostics, nondetErrors...)
	}
	return result, nil
}
