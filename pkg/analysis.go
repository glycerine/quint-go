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
	types, err := AnalyzeModuleTypes(modules, table)
	effects := NewEffectInferrer(table)
	modes := NewModeChecker()
	updates := MultipleUpdatesChecker{}
	nondet := NewNondetChecker(table)
	result := AnalysisResult{TypeAnalysisResult: types, Effects: effects.Effects, EffectErrors: effects.Errors, Modes: modes.Suggestions, Diagnostics: []Diagnostic{}}
	if err != nil {
		return result, err
	}
	for _, m := range types.Modules {
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
