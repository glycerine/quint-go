package cli

import (
	"strconv"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// Reporting predicates are Go backend instrumentation. Compile the public
// source view first so neither errors nor published IDs include those helpers.
func verifyCompileModules(front quint.FrontendResult, analysis quint.AnalysisResult, options quint.CompilationOptions, reports bool) (published, backend quint.CompilationResult, err error) {
	output := quint.AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}
	var privateOutput quint.AnalysisOutput
	var privateIDs *quint.IDGenerator
	var privateSources quint.SourceMap
	if reports {
		privateIDs = front.IDs.Clone()
		privateOutput, err = verifyCloneAnalysis(output)
		if err != nil {
			return published, backend, err
		}
		privateSources = make(quint.SourceMap, len(front.Sources))
		for id, location := range front.Sources {
			if location.End != nil {
				end := *location.End
				location.End = &end
			}
			privateSources[id] = location
		}
	}
	options.InvariantReports = false
	options.InvariantReportNames = nil
	published, err = quint.CompileModules(analysis.Modules, output, front.IDs, front.Sources, options)
	if err != nil || len(published.Diagnostics) > 0 || !reports {
		return published, published, err
	}
	options.InvariantReports = true
	options.InvariantReportNames = verifyReportNames(analysis.Modules, len(options.Invariants))
	backend, err = quint.CompileModules(analysis.Modules, privateOutput, privateIDs, privateSources, options)
	return published, backend, err
}

func verifyReportNames(modules []quint.Module, count int) []string {
	used := insdict.NewDict[string, bool]()
	for _, module := range modules {
		for _, declaration := range module.Declarations {
			used.Set(declaration.Name, true)
		}
	}
	names := make([]string, count)
	for i := range names {
		base := quint.InvariantReportName(i)
		name := base
		for suffix := 1; used.Get(name); suffix++ {
			name = base + "_" + strconv.Itoa(suffix)
		}
		used.Set(name, true)
		names[i] = name
	}
	return names
}

func verifyCloneAnalysis(output quint.AnalysisOutput) (quint.AnalysisOutput, error) {
	copy := quint.AnalysisOutput{Types: output.Types.Clone(), Effects: output.Effects.Clone(), Modes: output.Modes.Clone()}
	if copy.Types != nil {
		for id, scheme := range copy.Types.All() {
			typ, err := quint.ApplySubstitution(nil, scheme.Type)
			if err != nil {
				return quint.AnalysisOutput{}, err
			}
			scheme.Type = typ
			scheme.TypeVariables = append([]string(nil), scheme.TypeVariables...)
			scheme.RowVariables = append([]string(nil), scheme.RowVariables...)
			copy.Types.Set(id, scheme)
		}
	}
	if copy.Effects != nil {
		for id, scheme := range copy.Effects.All() {
			scheme.Effect = verifyCloneEffect(scheme.Effect)
			scheme.EffectVariables = append([]string(nil), scheme.EffectVariables...)
			scheme.EntityVariables = append([]string(nil), scheme.EntityVariables...)
			copy.Effects.Set(id, scheme)
		}
	}
	return copy, nil
}

func verifyCloneEffect(effect quint.Effect) quint.Effect {
	copy := effect
	if effect.Components != nil {
		copy.Components = append([]quint.EffectComponent{}, effect.Components...)
		for i := range copy.Components {
			copy.Components[i].Entity = verifyCloneEntity(copy.Components[i].Entity)
		}
	}
	if effect.Params != nil {
		copy.Params = make([]quint.Effect, len(effect.Params))
		for i, parameter := range effect.Params {
			copy.Params[i] = verifyCloneEffect(parameter)
		}
	}
	if effect.Result != nil {
		result := verifyCloneEffect(*effect.Result)
		copy.Result = &result
	}
	return copy
}

func verifyCloneEntity(entity quint.Entity) quint.Entity {
	copy := entity
	if entity.Reference != nil {
		id := *entity.Reference
		copy.Reference = &id
	}
	if entity.StateVariables != nil {
		copy.StateVariables = append([]quint.StateVariable{}, entity.StateVariables...)
	}
	if entity.Entities != nil {
		copy.Entities = make([]quint.Entity, len(entity.Entities))
		for i, child := range entity.Entities {
			copy.Entities[i] = verifyCloneEntity(child)
		}
	}
	return copy
}
