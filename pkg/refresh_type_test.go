package quint

import (
	"github.com/glycerine/insdict"
	"testing"
)

func TestRefreshTypeIDs(t *testing.T) {
	ids := NewIDGenerator()
	old := ids.Next()
	child := ids.Next()
	typ := Type{Kind: "set", ID: &old, Elem: &Type{Kind: "int", ID: &child}}
	sources := SourceMap{old: Location{}, child: Location{}}
	types := insdict.NewDict[string, TypeScheme]()
	types.Set(old.String(), TypeScheme{Type: Type{Kind: "int"}})
	out, err := RefreshTypeIDs(typ, ids, sources, AnalysisOutput{Types: types})
	if err != nil {
		t.Fatal(err)
	}
	if *out.ID == old || *out.Elem.ID == child || *out.ID == *out.Elem.ID {
		t.Fatal("IDs not refreshed")
	}
	if len(sources) != 4 || types.Len() != 2 {
		t.Fatal("metadata missing")
	}
	*out.Elem.ID = old
	if *typ.Elem.ID != child {
		t.Fatal("source aliased")
	}
	absent, err := RefreshTypeIDs(Type{Kind: "int"}, ids, nil, AnalysisOutput{})
	if err != nil || absent.ID != nil {
		t.Fatal(absent, err)
	}
}
