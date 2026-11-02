package quint

// LoadModulesWithSource loads and sorts transitive imports with a shared source
// map. On failure modules are nil, but spans allocated before the error remain
// available for resolving its reference. SyntaxErrors also identify their file.
// Locations use half-open UTF-16 spans, including parsed annotation types.
func LoadModulesWithSource(resolver SourceResolver, main SourceLookupPath) ([]Module, SourceMap, error) {
	result, err := loadModulesWithLocations(resolver, main, false)
	return result.Modules, result.Sources, err
}

// LoadModulesWithLocations returns UTF-16 and upstream-compatible source maps
// across all loaded files. Both maps remain available on failure; modules do not.
func LoadModulesWithLocations(resolver SourceResolver, main SourceLookupPath) (ModuleParseLocations, error) {
	return loadModulesWithLocations(resolver, main, true)
}

func loadModulesWithLocations(resolver SourceResolver, main SourceLookupPath, upstream bool) (ModuleParseLocations, error) {
	sources := SourceMap{}
	var raw UpstreamSourceMap
	if upstream {
		raw = UpstreamSourceMap{}
	}
	result := ModuleParseLocations{Sources: sources, UpstreamSources: raw}
	generator := NewIDGenerator()
	result.IDs = generator
	parse := func(text string, source SourceLookupPath) ([]Module, error) {
		file, err := NewSourceFile(source.SourceName, text)
		if err != nil {
			return nil, err
		}
		ids := &locatedParserIDs{generator: generator, file: file, sources: sources, upstreamSources: raw}
		modules, err := parseModulesWithIDs(text, ids)
		if err != nil {
			return nil, withSyntaxSource(err, source.SourceName)
		}
		if ids.err != nil {
			return nil, ids.err
		}
		return modules, nil
	}
	modules, err := loadSourceModules(resolver, main, parse)
	if err != nil {
		return result, err
	}
	sorted, diagnostics := SortModules(modules)
	if len(diagnostics) > 0 {
		return result, diagnostics[0]
	}
	result.Modules = sorted
	return result, nil
}
