package quint

import (
	"fmt"
	"strings"
)

// DiscoverTests selects operator definitions from a resolved module scope in
// collector insertion order. A nil match selects names ending in "Test".
// Matching uses the definition's unqualified Name, including for imported or
// instanced definitions. All qualifiers and hidden imports are eligible, just
// as in the reference CLI; the test evaluator decides pass/fail/ignore later.
//
// The caller supplies an already checked resolution and any custom matcher.
// JavaScript RegExp compilation belongs to the CLI adapter. The returned slice
// and namespace slices are owned; declarations/provenance retain references to
// the resolution and must be treated as read-only. Selecting an imported entry
// does not flatten it or map its name to a prepared evaluator's entry points.
func DiscoverTests(resolution LinkedNameResolution, module string, match func(string) bool) ([]LookupDefinition, error) {
	if resolution.Modules == nil {
		return nil, fmt.Errorf("test discovery requires linked module scopes")
	}
	scope := resolution.Modules.Get(module)
	if scope == nil {
		return nil, fmt.Errorf("test module %s does not exist", module)
	}
	if match == nil {
		match = func(name string) bool { return strings.HasSuffix(name, "Test") }
	}
	selected := []LookupDefinition{}
	for _, entries := range scope.Definitions.All() {
		for _, definition := range entries {
			if definition.Declaration != nil && definition.Declaration.Kind == "def" && match(definition.Name) {
				definition.Namespaces = append([]string(nil), definition.Namespaces...)
				selected = append(selected, definition)
			}
		}
	}
	return selected, nil
}

// NoTestsMatchedMessage mirrors the CLI explanation for an empty selection.
// Empty match has the same meaning as an omitted --match value in the reference.
func NoTestsMatchedMessage(match string) string {
	how := "By default, quint test runs definitions whose names end with \"Test\". Use --match to select other names (for example, --match '.*')"
	if match != "" {
		return "no tests matched --match '" + match + "'. " + how
	}
	return "no tests matched. " + how
}
