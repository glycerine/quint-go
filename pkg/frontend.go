package quint

import "github.com/glycerine/insdict"

// FrontendResult contains loaded, resolved, declaration-sorted modules and the
// source maps needed to report diagnostics. Diagnostics do not imply that the
// retained modules are valid for evaluation or compilation.
type FrontendResult struct {
	Modules         []Module
	Sources         SourceMap
	UpstreamSources UpstreamSourceMap
	Table           *insdict.Dict[ID, LookupDefinition]
	Scopes          *insdict.Dict[string, *NameScope]
	Diagnostics     []Diagnostic
}

// ParseProject loads a source graph and runs name resolution and declaration
// ordering. Name errors do not prevent sorting, matching the upstream phases.
// Source loading and syntax failures return an error with the partial source
// maps. Recovery from those failures is not yet supported by the Go parser.
// This entry point does not perform type/effect inference or unshadowing.
func ParseProject(resolver SourceResolver, main SourceLookupPath) (FrontendResult, error) {
	loaded, err := LoadModulesWithLocations(resolver, main)
	result := FrontendResult{Modules: loaded.Modules, Sources: loaded.Sources, UpstreamSources: loaded.UpstreamSources, Diagnostics: []Diagnostic{}}
	if err != nil {
		return result, err
	}
	linked, err := ResolveLinkedModules(loaded.Modules)
	if err != nil {
		return result, err
	}
	result.Table = linked.Table
	result.Scopes = linked.Modules
	result.Diagnostics = append(result.Diagnostics, linked.Errors...)
	var cycles []Diagnostic
	result.Modules, cycles = SortModuleDeclarations(loaded.Modules, linked.Table)
	result.Diagnostics = append(result.Diagnostics, cycles...)
	return result, nil
}

// UnusedDefinitions queries collected binding identities against the current
// lookup table, including any updates made by later transformation passes.
func (r FrontendResult) UnusedDefinitions(module string) []LookupDefinition {
	if r.Scopes == nil || r.Table == nil {
		return []LookupDefinition{}
	}
	return (LinkedNameResolution{Modules: r.Scopes, Table: r.Table}).UnusedDefinitions(module)
}
