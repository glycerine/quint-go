package quint

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestDecodedTypeValidationErrorPriority(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`{"kind":"fun","res":{"kind":"var"}}`, "var type missing name"},
		{`{"kind":"fun","arg":null,"res":{"kind":"const"}}`, "const type missing name"},
		{`{"kind":"fun","arg":{"kind":"var"},"extra":null}`, "var type missing name"},
		{`{"kind":"fun","arg":null,"extra":null}`, "unexpected fields in fun type"},
		{`{"kind":"app","args":[{"kind":"var"}],"ctor":{"kind":"int"}}`, "var type missing name"},
		{`{"kind":"app","args":[null],"ctor":{"kind":"int"}}`, `unknown type kind ""`},
		{`{"kind":"app","args":[],"ctor":{"kind":"const"}}`, "const type missing name"},
		{`{"kind":"app","args":[],"ctor":{"kind":"var","name":"a"}}`, "type application requires constant constructor"},
		{`{"kind":"sum","fields":{"kind":"var"}}`, "row variable missing name"},
		{`{"kind":"sum","fields":{"kind":"empty"},"extra":null}`, "unexpected fields in sum type"},
		{`{"kind":"rec","fields":{"kind":"row","fields":[{}],"other":{"kind":"empty"}}}`, `unknown type kind ""`},
		{`{"kind":"rec","fields":{"kind":"row","fields":[{}],"other":{"kind":"var"}}}`, "row variable missing name"},
		{`{"kind":"list","elem":{"kind":"int"},"Elem":null}`, "unexpected fields in list type"},
	} {
		before := typeDecodeReceiver()
		got := typeDecodeReceiver()
		err := json.Unmarshal([]byte(tc.input), &got)
		if err == nil || err.Error() != tc.want || !reflect.DeepEqual(got, before) {
			t.Fatalf("decode %s: got %v, want %q; receiver unchanged=%v", tc.input, err, tc.want, reflect.DeepEqual(got, before))
		}
	}
}

func TestDecodedRowValidationErrorPriority(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`{"kind":"row","fields":[{}]}`, `unknown type kind ""`},
		{`{"kind":"row","fields":[null],"other":{"kind":"empty"}}`, `unknown type kind ""`},
		{`{"kind":"row","fields":[{"fieldName":"x"}],"other":{"kind":"empty"}}`, `unknown type kind ""`},
		{`{"kind":"row","fields":[{}, {"fieldType":{"kind":"var"}}]}`, "var type missing name"},
		{`{"kind":"row","fields":[{}],"other":{"kind":"var"}}`, "row variable missing name"},
		{`{"kind":"row","fields":[{}],"other":{"kind":"empty"},"extra":null}`, "unexpected fields in row row"},
		{`{"kind":"row","fields":[{}],"other":{"kind":"row"}}`, "row missing tail"},
		{`{"kind":"row","fields":[{"fieldType":null}],"extra":null}`, `unknown type kind ""`},
		{`{"kind":"row","fields":[{}, {"FieldType":{"kind":"int"}}]}`, "unexpected fields in row field"},
		{`{"kind":"row","fields":[{"fieldType":{"kind":"int"}}]}`, "row missing tail"},
	} {
		before := rowDecodeReceiver()
		got := rowDecodeReceiver()
		err := json.Unmarshal([]byte(tc.input), &got)
		if err == nil || err.Error() != tc.want || !reflect.DeepEqual(got, before) {
			t.Fatalf("decode %s: got %v, want %q; receiver unchanged=%v", tc.input, err, tc.want, reflect.DeepEqual(got, before))
		}
	}
}

func TestDecodedTypesRetainLegacyJSONForms(t *testing.T) {
	for _, tc := range []struct{ input, name string }{
		{`{"kind":"unknown","kind":"var","name":"a"}`, "a"},
		{`{"kind":"var","name":false,"name":"a"}`, "a"},
		{`{"kind":"var","name":"first","\u006eame":"last"}`, "last"},
		{"{\"kind\":\"var\",\"name\":\"\xff\"}", "\ufffd"},
	} {
		var got Type
		if err := json.Unmarshal([]byte(tc.input), &got); err != nil || got.Kind != "var" || got.Name != tc.name {
			t.Fatalf("legacy type %q: %#v, %v", tc.input, got, err)
		}
	}
	var list Type
	if err := json.Unmarshal([]byte(`{"kind":"list","elem":{"kind":"var"},"elem":{"kind":"int"}}`), &list); err != nil || list.Elem.Kind != "int" {
		t.Fatal("last raw child must win", list, err)
	}
	var row Row
	if err := json.Unmarshal([]byte(`{"kind":"row","fields":[{}],"fields":[],"other":{"kind":"empty"}}`), &row); err != nil || row.Fields == nil || len(row.Fields) != 0 {
		t.Fatal("last raw fields must win", row, err)
	}
	var field RowField
	if err := json.Unmarshal([]byte(`{}`), &field); err != nil || !reflect.DeepEqual(field, RowField{}) {
		t.Fatal("standalone empty row field remains accepted", field, err)
	}
	for _, input := range []string{`{}`, `null`, `{"fieldName":"renamed"}`} {
		// Partial standalone row-field decoding must even retain a caller's
		// invalid hand-built type; its containing Row is the validation boundary.
		field := RowField{Name: "before", Type: Type{Kind: "list"}}
		if err := json.Unmarshal([]byte(input), &field); err != nil || field.Type.Kind != "list" || field.Type.Elem != nil {
			t.Fatal("standalone partial row field changed validation", field, err)
		}
	}
}

func TestPublicTypeValidationStillWalksHandBuiltTrees(t *testing.T) {
	for _, tc := range []struct {
		value Type
		want  string
	}{
		{Type{Kind: "fun", Arg: &Type{Kind: "var"}}, "var type missing name"},
		{Type{Kind: "fun", Res: &Type{Kind: "var"}}, "fun type missing child"},
		{Type{Kind: "oper", Args: []Type{{Kind: "var"}}}, "var type missing name"},
		{Type{Kind: "app", Args: []Type{{Kind: "var"}}, Ctor: &Type{Kind: "int"}}, "var type missing name"},
		{Type{Kind: "app", Ctor: &Type{Kind: "const"}}, "const type missing name"},
		{Type{Kind: "sum", Fields: &Row{Kind: "var"}}, "sum requires concrete row"},
		{Type{Kind: "rec", Fields: &Row{Kind: "row", Fields: []RowField{{Type: Type{Kind: "const"}}}}}, "const type missing name"},
	} {
		if err := tc.value.Validate(); err == nil || err.Error() != tc.want {
			t.Fatalf("public validation %#v: got %v, want %q", tc.value, err, tc.want)
		}
	}
	leaf := Type{Kind: "list", Elem: &Type{Kind: "int"}}
	for range 64 {
		previous := leaf
		leaf = Type{Kind: "list", Elem: &previous}
	}
	if err := leaf.Validate(); err != nil {
		t.Fatal(err)
	}
	last := &leaf
	for last.Elem != nil {
		last = last.Elem
	}
	last.Kind = "invalid"
	if err := leaf.Validate(); err == nil || err.Error() != `unknown type kind "invalid"` {
		t.Fatal("public validation stopped recursing", err)
	}
	row := rowDecodeReceiver()
	row.Fields[0].Type.Elem = nil
	row.Other.Kind = "invalid"
	if err := row.Validate(); err == nil || err.Error() != "list type missing child" {
		t.Fatal("public row validation changed field-before-tail order", err)
	}
}

func typeDecodeReceiver() Type {
	return Type{Kind: "fun", Arg: &Type{Kind: "list", Elem: &Type{Kind: "int"}}, Res: &Type{Kind: "str"}}
}

func rowDecodeReceiver() Row {
	return Row{Kind: "row", Fields: []RowField{{Name: "original", Type: Type{Kind: "list", Elem: &Type{Kind: "int"}}}}, Other: &Row{Kind: "var", Name: "r"}}
}

func FuzzDecodedTypeValidation(f *testing.F) {
	for _, input := range []string{
		`{"kind":"int"}`, `{"kind":"list","elem":{"kind":"str"}}`,
		`{"kind":"fun","res":{"kind":"var"}}`, `{"kind":"int","kind":"str"}`,
		`{"kind":"rec","fields":{"kind":"row","fields":[{}]}}`,
		`{"kind":"int","Kind":null}`, `{"kind":"oper","args":[null]}`,
		`null`, `[]`, `{`, `{"kind":"int"} {}`, "{\"kind\":\"var\",\"name\":\"\xff\"}",
	} {
		f.Add([]byte(input), false)
	}
	for _, input := range []string{
		`{"kind":"empty"}`, `{"kind":"row","fields":[],"other":{"kind":"empty"}}`,
		`{"kind":"row","fields":[{}],"other":{"kind":"empty"}}`,
		`{"kind":"row","fields":[null],"other":{"kind":"var"}}`,
		`{"kind":"row","fields":[{"fieldType":{"kind":"bool"}}],"other":{"kind":"empty"}}`,
		`{"kind":"row","fields":[],"fields":[{}],"other":{"kind":"empty"}}`,
		`null`, `{"kind":"var","Name":"r"}`, "{\"kind\":\"var\",\"name\":\"\xff\"}",
	} {
		f.Add([]byte(input), true)
	}
	f.Fuzz(func(t *testing.T, data []byte, row bool) {
		if row {
			before, got := rowDecodeReceiver(), rowDecodeReceiver()
			if err := json.Unmarshal(data, &got); err != nil {
				if !reflect.DeepEqual(got, before) {
					t.Fatal("failed row decode mutated receiver")
				}
			} else if err := got.Validate(); err != nil {
				t.Fatal("decoded row failed public recursive validation", err)
			}
		} else {
			before, got := typeDecodeReceiver(), typeDecodeReceiver()
			if err := json.Unmarshal(data, &got); err != nil {
				if !reflect.DeepEqual(got, before) {
					t.Fatal("failed type decode mutated receiver")
				}
			} else if err := got.Validate(); err != nil {
				t.Fatal("decoded type failed public recursive validation", err)
			}
		}
	})
}

func BenchmarkDecodeTypeValidation(b *testing.B) {
	for _, depth := range []int{1, 16, 64, 256} {
		typeJSON := strings.Repeat(`{"kind":"list","elem":`, depth) + `{"kind":"int"}` + strings.Repeat(`}`, depth)
		rowJSON := strings.Repeat(`{"kind":"row","fields":[{"fieldName":"x","fieldType":{"kind":"int"}}],"other":`, depth) + `{"kind":"empty"}` + strings.Repeat(`}`, depth)
		b.Run("type/depth"+strconv.Itoa(depth), func(b *testing.B) {
			data := []byte(typeJSON)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				var got Type
				if err := json.Unmarshal(data, &got); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("row/depth"+strconv.Itoa(depth), func(b *testing.B) {
			data := []byte(rowJSON)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				var got Row
				if err := json.Unmarshal(data, &got); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
