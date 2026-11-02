package quint

import (
	"encoding/json"
	"testing"
)

func TestBindTypeOccursCheck(t *testing.T) {
	for _, text := range []string{"a", "List[a]", "Set[a]", "a -> int", "(int,a)=>bool", "(int,a)", "{x:List[a]}", "Box[a]"} {
		typ, err := ParseType(text)
		if err != nil {
			t.Fatal(err)
		}
		subs, err := BindTypeVariable("a", typ)
		want := "Can't bind a to " + TypeString(typ) + ": cyclical binding"
		if err == nil || err.Error() != want || subs != nil {
			t.Fatalf("%s: %v %+v", text, err, subs)
		}
	}
	for _, text := range []string{"int", "b", "{x:int|a}", "List[b]"} {
		typ, err := ParseType(text)
		if err != nil {
			t.Fatal(err)
		}
		subs, err := BindTypeVariable("a", typ)
		if err != nil || len(subs) != 1 || subs[0].Kind != "type" || subs[0].Name != "a" {
			t.Fatalf("%s: %v %+v", text, err, subs)
		}
	}
}

func TestBindRowOccursCheck(t *testing.T) {
	for _, row := range []Row{
		{Kind: "var", Name: "r"},
		{Kind: "row", Fields: []RowField{{Name: "x", Type: Type{Kind: "int"}}}, Other: &Row{Kind: "var", Name: "r"}},
	} {
		subs, err := BindRowVariable("r", row)
		if err == nil || err.Error() != "Can't bind r to "+RowString(row)+": cyclical binding" || subs != nil {
			t.Fatalf("%v %+v", err, subs)
		}
	}
	typ, err := ParseType("{x:r, y:{z:int|r}|s}")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(typ)
	subs, err := BindRowVariable("r", *typ.Fields)
	if err != nil || len(subs) != 1 || subs[0].Kind != "row" {
		t.Fatalf("%v %+v", err, subs)
	}
	subs[0].Row.Fields[1].Type.Fields.Fields[0].Type.Kind = "str"
	after, _ := json.Marshal(typ)
	if string(before) != string(after) {
		t.Fatal("row binding aliases input")
	}
}

func TestBindingsCopyTypesAndRejectMalformedIR(t *testing.T) {
	typ, _ := ParseType("List[int]")
	subs, err := BindTypeVariable("a", typ)
	if err != nil {
		t.Fatal(err)
	}
	subs[0].Type.Elem.Kind = "str"
	if typ.Elem.Kind != "int" {
		t.Fatal("type binding aliases input")
	}
	if _, err = BindTypeVariable("a", Type{Kind: "list"}); err == nil {
		t.Fatal("malformed type accepted")
	}
	if _, err = BindRowVariable("r", Row{Kind: "row"}); err == nil {
		t.Fatal("malformed row accepted")
	}
}
