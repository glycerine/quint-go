package quint

import "github.com/glycerine/insdict"

// AliasInliningResult keeps modules, lookup payloads, and inferred types in
// agreement after nominal aliases have been expanded for compilation.
type AliasInliningResult struct {
	Modules  []Module
	Table    *insdict.Dict[ID, LookupDefinition]
	Analysis AnalysisOutput
}

// InlineTypeAliases expands aliases throughout a resolved project. Effects and
// modes are retained; module trees, lookup payloads, and type schemes are copied.
// Type applications should have been expanded by analysis before this pass.
func InlineTypeAliases(modules []Module, table *insdict.Dict[ID, LookupDefinition], analysis AnalysisOutput) (AliasInliningResult, error) {
	result := AliasInliningResult{Modules: make([]Module, len(modules)), Table: NewExpressionNameResolver(nil).Table, Analysis: analysis}
	inline := func(t Type) (Type, error) { return InlineTypeAliasesInType(table, t) }
	for i, m := range modules {
		out := m
		out.Declarations = make([]Declaration, len(m.Declarations))
		for j, d := range m.Declarations {
			value, err := TransformDeclarationTypes(d, inline)
			if err != nil {
				return AliasInliningResult{}, err
			}
			out.Declarations[j] = value
		}
		result.Modules[i] = out
	}
	for id, def := range table.All() {
		if def.Declaration != nil {
			d, err := TransformDeclarationTypes(*def.Declaration, inline)
			if err != nil {
				return AliasInliningResult{}, err
			}
			def.Declaration = &d
		} else if def.Parameter != nil {
			p := *def.Parameter
			if p.TypeAnnotation != nil {
				t, err := inline(*p.TypeAnnotation)
				if err != nil {
					return AliasInliningResult{}, err
				}
				p.TypeAnnotation = &t
			}
			def.Parameter = &p
		}
		result.Table.Set(id, def)
	}
	if analysis.Types != nil {
		result.Analysis.Types = insdict.NewDict[string, TypeScheme]()
		for id, scheme := range analysis.Types.All() {
			t, err := inline(scheme.Type)
			if err != nil {
				return AliasInliningResult{}, err
			}
			scheme.Type = t
			result.Analysis.Types.Set(id, scheme)
		}
	}
	return result, nil
}
