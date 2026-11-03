package lsp

import (
	"encoding/json"
	"reflect"
	"testing"
)

type symbolMeaning struct {
	Name     string
	Kind     int
	Children []symbolMeaning
}

func symbolMeanings(symbols []semanticSymbol) []symbolMeaning {
	out := []symbolMeaning{}
	for _, symbol := range symbols {
		out = append(out, symbolMeaning{Name: symbol.Name, Kind: symbol.Kind, Children: symbolMeanings(symbol.Children)})
	}
	return out
}

func TestUpstreamDocumentSymbolClassificationsAndFilters(t *testing.T) {
	t.Parallel()
	_, fixtures := hoverSymbolFixtures(t)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			snapshot := hoverSymbolSnapshot(t, fixture)
			got := semanticSymbolsForSnapshot(snapshot, buildSemanticIndex(snapshot))
			var want []semanticSymbol
			if err := json.Unmarshal(fixture.Result, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(symbolMeanings(got), symbolMeanings(want)) {
				t.Fatal(symbolMeanings(got), symbolMeanings(want))
			}
			var check func([]semanticSymbol)
			check = func(symbols []semanticSymbol) {
				for _, symbol := range symbols {
					if semanticTestRangeText(t, snapshot, Location{URI: snapshot.URI, Range: symbol.SelectionRange}) != symbol.Name {
						t.Fatal("invalid selection range", symbol)
					}
					check(symbol.Children)
				}
			}
			check(got)
		})
	}
}
