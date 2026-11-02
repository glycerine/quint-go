package quint

import "strings"

// LoadSourceModules parses a source and its transitive file imports. Modules are
// returned in discovery order (imported files' modules in reverse source order,
// matching upstream's worklist). Dependency sorting/name resolution are separate.
// Errors are fail-fast; no partial module list is returned.
func LoadSourceModules(resolver SourceResolver, main SourceLookupPath) ([]Module, error) {
	text, err := resolver.Load(main)
	if err != nil {
		return nil, err
	}
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs(text, ids)
	if err != nil {
		return nil, err
	}
	type work struct {
		module Module
		path   SourceLookupPath
	}
	queue := []work{}
	for _, m := range modules {
		queue = append(queue, work{m, main})
	}
	visited := map[string]string{strings.ToLower(main.NormalizedPath): main.NormalizedPath}
	for next := 0; next < len(queue); next++ {
		current := queue[next]
		for _, decl := range current.module.Declarations {
			if (decl.Kind != "import" && decl.Kind != "instance") || decl.FromSource == nil || *decl.FromSource == "" {
				continue
			}
			target := resolver.LookupPath(resolver.StemPath(current.path), *decl.FromSource+".qnt")
			lower := strings.ToLower(target.NormalizedPath)
			if original, ok := visited[lower]; ok {
				if original == target.NormalizedPath {
					continue
				}
				return nil, Diagnostic{Code: QNT408, Message: "Importing two files that only differ in case: " + original + " vs. " + target.NormalizedPath + ". Choose one way.", Reference: &decl.ID}
			}
			source, err := resolver.Load(target)
			if err != nil {
				return nil, Diagnostic{Code: QNT013, Message: "import ... from '" + *decl.FromSource + "': could not load", Reference: &decl.ID}
			}
			imported, err := parseModulesWithIDs(source, ids)
			if err != nil {
				return nil, err
			}
			visited[lower] = target.NormalizedPath
			for i := len(imported) - 1; i >= 0; i-- {
				modules = append(modules, imported[i])
				queue = append(queue, work{imported[i], target})
			}
		}
	}
	return modules, nil
}
