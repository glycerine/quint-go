package cli

import (
	"reflect"

	quint "github.com/glycerine/quint-go/pkg"
)

type cliMainSourceResolver struct {
	*quint.FileSourceResolver
	main string
}

func (resolver cliMainSourceResolver) Load(path quint.SourceLookupPath) (string, error) {
	if path.NormalizedPath == resolver.main {
		return resolver.LoadPreservingLineEnds(path)
	}
	return resolver.FileSourceResolver.Load(path)
}

// Upstream keeps all diagnostics in JSON but removes deeply equal error
// messages before rendering terminal output. Preserve the first occurrence.
func cliUniqueErrorMessages(messages []quint.ErrorMessage) []quint.ErrorMessage {
	unique := make([]quint.ErrorMessage, 0, len(messages))
	for _, message := range messages {
		duplicate := false
		for _, previous := range unique {
			if reflect.DeepEqual(message, previous) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			unique = append(unique, message)
		}
	}
	return unique
}
