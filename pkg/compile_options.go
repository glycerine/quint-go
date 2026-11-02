package quint

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GuessMainModule follows explicit flag, source default, then filename priority.
func GuessMainModule(explicit, sourceDefault, input string) string {
	if explicit != "" {
		return explicit
	}
	if sourceDefault != "" {
		return sourceDefault
	}
	return strings.TrimSuffix(filepath.Base(input), ".qnt")
}

// SelectMainModule reports the same missing-module condition as compile.
func SelectMainModule(modules []Module, explicit, sourceDefault, input string) (int, error) {
	name := GuessMainModule(explicit, sourceDefault, input)
	for i, m := range modules {
		if m.Name == name {
			return i, nil
		}
	}
	return -1, fmt.Errorf("module %s does not exist", name)
}

// CompilationInvariants combines the legacy single flag and repeated flags.
// Only the legacy literal true is omitted; list entries retain their order.
func CompilationInvariants(single string, multiple []string) (string, []string) {
	list := []string{}
	if single != "" && single != "true" {
		list = append(list, single)
	}
	list = append(list, multiple...)
	if len(list) == 0 {
		return "true", list
	}
	return strings.Join(list, " and "), list
}
