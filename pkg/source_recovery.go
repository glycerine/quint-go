package quint

import (
	"fmt"

	"github.com/glycerine/insdict"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// LoadModulesRecovering runs source-resolution phase two while preserving the
// main source's accumulated parser diagnostics and available source locations.
// It matches the reference's error contract: imported sources' parser
// diagnostics are not appended, and a missing/case-conflicting import returns
// the original main modules rather than the partially expanded module graph.
// Existing fail-fast loading entry points keep their original behavior.
func LoadModulesRecovering(resolver SourceResolver, main SourceLookupPath) (ModuleRecoveryResult, error) {
	result := ModuleRecoveryResult{ModuleParseLocations: ModuleParseLocations{Sources: SourceMap{}, UpstreamSources: UpstreamSourceMap{}, IDs: NewIDGenerator()}, Diagnostics: []Diagnostic{}}
	if resolver == nil {
		return result, fmt.Errorf("source resolver is required")
	}
	text, err := resolver.Load(main)
	if err != nil {
		return result, err
	}
	first, err := parseModulesRecoveringWithIDs(main.SourceName, text, result.IDs)
	if err != nil {
		return first, err
	}
	result = first
	mainModules := first.Modules
	type work struct {
		module Module
		source SourceLookupPath
	}
	queue := []work{}
	for _, module := range mainModules {
		queue = append(queue, work{module, main})
	}
	bySource := insdict.NewDict[string, []Module]()
	bySource.Set(main.NormalizedPath, mainModules)
	lowerPath := cases.Lower(language.Und).String
	visited := insdict.NewDict[string, string]()
	visited.Set(lowerPath(main.NormalizedPath), main.NormalizedPath)
	fail := func(code, message string, reference ID) (ModuleRecoveryResult, error) {
		result.Modules = mainModules
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: code, Message: message, Reference: &reference})
		return result, nil
	}
	for next := 0; next < len(queue); next++ {
		current := queue[next]
		for _, decl := range current.module.Declarations {
			if (decl.Kind != "import" && decl.Kind != "instance") || decl.FromSource == nil || *decl.FromSource == "" {
				continue
			}
			target := resolver.LookupPath(resolver.StemPath(current.source), *decl.FromSource+".qnt")
			lower := lowerPath(target.NormalizedPath)
			if previous, ok := visited.Get2(lower); ok {
				if previous == target.NormalizedPath {
					continue
				}
				original := lower
				for _, name := range visited.All() {
					if lowerPath(name) == lower && name != lower {
						original = name
						break
					}
				}
				return fail(QNT408, "Importing two files that only differ in case: "+original+" vs. "+target.NormalizedPath+". Choose one way.", decl.ID)
			}
			text, err := resolver.Load(target)
			if err != nil {
				return fail(QNT013, "import ... from '"+*decl.FromSource+"': could not load", decl.ID)
			}
			imported, err := parseModulesRecoveringWithIDs(target.SourceName, text, result.IDs)
			for id, location := range imported.Sources {
				result.Sources[id] = location
			}
			for id, location := range imported.UpstreamSources {
				result.UpstreamSources[id] = location
			}
			if err != nil {
				return result, err
			}
			reversed := make([]Module, len(imported.Modules))
			for i, module := range imported.Modules {
				reversed[len(reversed)-1-i] = module
			}
			for _, module := range reversed {
				queue = append(queue, work{module, target})
			}
			bySource.Set(target.NormalizedPath, reversed)
			visited.Set(lower, target.NormalizedPath)
		}
	}
	modules := []Module{}
	for _, sourceModules := range bySource.All() {
		modules = append(modules, sourceModules...)
	}
	sorted, diagnostics := SortModules(modules)
	result.Modules = sorted
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	return result, nil
}

// ParseProjectRecovering runs name resolution and declaration ordering on the
// recovered source graph, accumulating diagnostics across all four frontend
// phases. Infrastructure/parser-conversion failures return an error; language
// diagnostics remain in the result. Callers must check Diagnostics before
// compiling or evaluating the recovered modules.
func ParseProjectRecovering(resolver SourceResolver, main SourceLookupPath) (FrontendResult, error) {
	loaded, err := LoadModulesRecovering(resolver, main)
	result := FrontendResult{Modules: loaded.Modules, IDs: loaded.IDs, Sources: loaded.Sources, UpstreamSources: loaded.UpstreamSources, Diagnostics: append([]Diagnostic{}, loaded.Diagnostics...)}
	if err != nil {
		return result, err
	}
	// The reference collector annotates non-operator declarations in place;
	// operator definitions are copied before collection. Preserve that phase-
	// three metadata on the recovered project without changing parser output.
	for i := range loaded.Modules {
		for j := range loaded.Modules[i].Declarations {
			declaration := &loaded.Modules[i].Declarations[j]
			switch declaration.Kind {
			case "const", "var", "typedef", "assume":
				if declaration.Depth == nil && declaration.Name != "_" && !builtinResolutionNames[declaration.Name] {
					depth := 0
					declaration.Depth = &depth
				}
			}
		}
	}
	linked, err := ResolveLinkedModules(loaded.Modules)
	if err != nil {
		return result, err
	}
	result.Table, result.Scopes = linked.Table, linked.Modules
	result.Diagnostics = append(result.Diagnostics, linked.Errors...)
	var cycles []Diagnostic
	result.Modules, cycles = SortModuleDeclarations(loaded.Modules, linked.Table)
	result.Diagnostics = append(result.Diagnostics, cycles...)
	return result, nil
}
