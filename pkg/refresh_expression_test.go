package quint

import (
	"github.com/glycerine/insdict"
	"testing"
)

func TestRefreshExpressionIDs(t *testing.T) {
	ids := NewIDGenerator()
	e, err := parseExpressionWithIDs("(x)=>x+1", ids)
	if err != nil {
		t.Fatal(err)
	}
	types := insdict.NewDict[string, TypeScheme]()
	effects := insdict.NewDict[string, EffectScheme]()
	modes := insdict.NewDict[string, Qualifier]()
	types.Set(e.ID.String(), TypeScheme{Type: Type{Kind: "int"}})
	effects.Set(e.ID.String(), EffectScheme{Effect: Effect{Kind: "concrete"}})
	modes.Set(e.ID.String(), PureDef)
	sources := SourceMap{}
	out := RefreshExpressionIDs(e, ids, sources, AnalysisOutput{Types: types, Effects: effects, Modes: modes})
	if out.ID == e.ID || out.Params[0].ID == e.Params[0].ID || out.Body.ID == e.Body.ID {
		t.Fatal("IDs retained")
	}
	if types.Len() != 2 || effects.Len() != 2 || modes.Get(out.ID.String()) != PureDef {
		t.Fatal("metadata not copied")
	}
	out.Params[0].Name = "changed"
	if e.Params[0].Name != "x" {
		t.Fatal("input mutated")
	}
	// Missing optional metadata does not prevent refreshing.
	again := RefreshExpressionIDs(e, ids, nil, AnalysisOutput{})
	if again.ID == out.ID {
		t.Fatal("generator not advanced")
	}
}
