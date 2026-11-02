package quint

import "errors"

// LoadModulesWithSource loads and sorts transitive imports with a shared source
// map. On failure modules are nil, but spans allocated before the error remain
// available for resolving its reference. SyntaxErrors also identify their file.
// Locations use half-open UTF-16 spans; type IDs are not yet assigned.
func LoadModulesWithSource(resolver SourceResolver, main SourceLookupPath) ([]Module, SourceMap, error) {
	sources := SourceMap{}
	generator := NewIDGenerator()
	parse := func(text string, source SourceLookupPath) ([]Module, error) {
		file, err := NewSourceFile(source.SourceName, text)
		if err != nil {
			return nil, err
		}
		ids := &locatedParserIDs{generator: generator, file: file, sources: sources}
		modules, err := parseModulesWithIDs(text, ids)
		if err != nil {
			var syntax *SyntaxErrors
			if errors.As(err, &syntax) {
				syntax.Source = source.SourceName
			}
			return nil, err
		}
		if ids.err != nil {
			return nil, ids.err
		}
		return modules, nil
	}
	modules, err := loadSourceModules(resolver, main, parse)
	if err != nil {
		return nil, sources, err
	}
	sorted, diagnostics := SortModules(modules)
	if len(diagnostics) > 0 {
		return nil, sources, diagnostics[0]
	}
	return sorted, sources, nil
}
