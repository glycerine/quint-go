package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestResolveDiagnostic(t *testing.T) {
	a, _ := ParseID("1")
	b, _ := ParseID("2")
	missing, _ := ParseID("3")
	la := Location{Source: "a", Start: Position{Line: 1}}
	lb := Location{Source: "b"}
	sources := SourceMap{a: la, b: lb}
	cases := []struct {
		d                Diagnostic
		locations, trace []Location
	}{
		{Diagnostic{}, []Location{}, nil},
		{Diagnostic{Reference: &a}, []Location{la}, nil},
		{Diagnostic{Reference: &missing}, []Location{}, nil},
		{Diagnostic{Reference: &a, Trace: []ID{b, missing, a, b}}, []Location{lb}, []Location{la, lb}},
		{Diagnostic{Reference: &a, Trace: []ID{missing, b}}, []Location{}, []Location{lb}},
	}
	for _, c := range cases {
		c.d.Code = "QNT404"
		c.d.Message = "unknown name"
		got := ResolveDiagnostic(sources, c.d)
		if got.Explanation != "[QNT404] unknown name" || !reflect.DeepEqual(got.Locations, c.locations) || !reflect.DeepEqual(got.TraceLocations, c.trace) {
			t.Fatalf("resolve %+v: %+v", c.d, got)
		}
	}
}
func TestErrorTrees(t *testing.T) {
	leaf := ErrorLeaf("in argument", "type mismatch")
	if got := WrapError("in argument", leaf); !reflect.DeepEqual(got, leaf) {
		t.Fatal("duplicate context retained")
	}
	tree := ErrorTree{Location: "in call", Children: []ErrorTree{leaf, WrapError("in result", ErrorLeaf("in value", "invalid"))}}
	want := "type mismatch\nin argument\nand\ninvalid\nin value\nin result\nin call\n"
	if got := tree.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := ErrorLeaf("here", "").String(); got != "here\n" {
		t.Fatalf("empty message: %q", got)
	}
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ErrorTree
	if err = json.Unmarshal(data, &decoded); err != nil || !reflect.DeepEqual(tree, decoded) {
		t.Fatalf("round trip: %v %v", decoded, err)
	}
}

func TestDiagnosticDataRoundTrip(t *testing.T) {
	leaf := ErrorLeaf("in expression", "wrong type")
	d := Diagnostic{Code: QNT006, Message: "use equality", Data: &DiagnosticData{Fix: &DiagnosticFix{Kind: "replace", Original: "=", Replacement: "=="}, Trace: &leaf}}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var got Diagnostic
	if err = json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(d, got) {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	data, err = json.Marshal(DiagnosticData{})
	if err != nil || string(data) != "{}" {
		t.Fatalf("optional fields: %s %v", data, err)
	}
}
