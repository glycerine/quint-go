package lsp

import quint "github.com/glycerine/quint-go/pkg"

// The outline classifies source qualifiers as the upstream editor does. Types,
// assumptions and import directives remain outside its declaration outline.
func documentSymbolKind(declaration quint.Declaration) int {
	switch declaration.Kind {
	case "const":
		return 14 // Constant
	case "var":
		return 13 // Variable
	case "def":
		switch declaration.Qualifier {
		case quint.Def:
			return 6 // Method
		case quint.Val, quint.PureVal:
			return 8 // Field
		case quint.Action:
			return 24 // Event
		case quint.Run:
			return 7 // Property
		default:
			return 12 // Function, including pure def/nondet/temporal
		}
	}
	return 0
}

func semanticSymbolsForSnapshot(snapshot *Snapshot, index semanticIndex) []semanticSymbol {
	out := []semanticSymbol{}
	for _, module := range snapshot.Frontend.Modules {
		location, ok := snapshot.Frontend.Sources[module.ID]
		if !ok || uriForSource(location.Source) != snapshot.URI {
			continue
		}
		selection, ok := semanticTokenRange(snapshot, module.ID, module.Name)
		if !ok {
			continue
		}
		symbol := semanticSymbol{Name: module.Name, Kind: 2, Range: semanticRange(location), SelectionRange: selection.Range, Children: []semanticSymbol{}}
		// The analyzed declaration order is observable in the upstream outline.
		// Do not reorder dependency-sorted declarations into lexical order.
		for _, declaration := range module.Declarations {
			kind := documentSymbolKind(declaration)
			definition, ok := index.definitions.Get2(declaration.ID.String())
			if kind == 0 || !ok || definition.Location.URI != snapshot.URI {
				continue
			}
			symbol.Children = append(symbol.Children, semanticSymbol{Name: declaration.Name, Kind: kind, Range: definition.FullRange, SelectionRange: definition.Location.Range})
		}
		out = append(out, symbol)
	}
	return out
}
