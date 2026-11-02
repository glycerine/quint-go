package lsp

import (
	"encoding/json"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// diagnosticsBySource follows reporting.ts:diagnosticsFromErrors. The error's
// reference owns the diagnostic; trace frames never move it to another file.
// Unknown references are omitted, and group/error order follows the input.
func diagnosticsBySource(sources quint.SourceMap, diagnostics []quint.Diagnostic) *insdict.Dict[string, []Diagnostic] {
	groups := insdict.NewDict[string, []Diagnostic]()
	for _, diagnostic := range diagnostics {
		if diagnostic.Reference == nil {
			continue
		}
		location, found := sources[*diagnostic.Reference]
		if !found {
			continue
		}
		converted := Diagnostic{
			Range: rangeForLocation(location), Severity: 1,
			Code: diagnostic.Code, Source: "quint", Message: diagnostic.Message,
		}
		if diagnostic.Data != nil {
			// Marshal a value copy rather than retaining a pointer into analysis.
			// A fix stays with its origin group, so selecting another document
			// cannot accidentally attach an imported edit to the open buffer.
			converted.Data, _ = json.Marshal(diagnostic.Data)
		}
		groups.Set(location.Source, append(groups.Get(location.Source), converted))
	}
	return groups
}

func diagnosticsForSource(groups *insdict.Dict[string, []Diagnostic], source string) []Diagnostic {
	if groups != nil {
		if diagnostics, found := groups.Get2(source); found {
			return diagnostics
		}
	}
	// LSP requires an empty array to clear a document's previous diagnostics.
	return []Diagnostic{}
}
