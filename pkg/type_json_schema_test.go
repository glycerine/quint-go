package quint

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTypeJSONVariantFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind, input, allowed string
	}{
		{"bool", `{"kind":"bool"}`, ""},
		{"int", `{"kind":"int"}`, ""},
		{"str", `{"kind":"str"}`, ""},
		{"const", `{"kind":"const","name":"T"}`, "name"},
		{"var", `{"kind":"var","name":"a"}`, "name"},
		{"set", `{"kind":"set","elem":{"kind":"int"}}`, "elem"},
		{"list", `{"kind":"list","elem":{"kind":"str"}}`, "elem"},
		{"fun", `{"kind":"fun","arg":{"kind":"int"},"res":{"kind":"bool"}}`, "arg res"},
		{"oper", `{"kind":"oper","args":[],"res":{"kind":"bool"}}`, "args res"},
		{"app", `{"kind":"app","args":[],"ctor":{"kind":"const","name":"T"}}`, "args ctor"},
		{"tup", `{"kind":"tup","fields":{"kind":"empty"}}`, "fields"},
		{"rec", `{"kind":"rec","fields":{"kind":"var","name":"r"}}`, "fields"},
		{"sum", `{"kind":"sum","fields":{"kind":"row","fields":[],"other":{"kind":"empty"}}}`, "fields"},
	}
	fields := []string{"name", "elem", "arg", "res", "args", "fields", "ctor", "other", "value", "params", "extra", "Kind", "ID"}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			for _, id := range []string{"", `,"id":0`, `,"id":340282366920938463463374607431768211456`, `,"id":null`} {
				input := strings.TrimSuffix(tc.input, "}") + id + "}"
				var got Type
				if err := json.Unmarshal([]byte(input), &got); err != nil {
					t.Fatal(input, err)
				}
				if (got.ID == nil) != (id == "" || strings.HasSuffix(id, "null")) {
					t.Fatal("optional ID presence", input, got.ID)
				}
				encoded, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				var roundtrip Type
				if err = json.Unmarshal(encoded, &roundtrip); err != nil || !reflect.DeepEqual(got, roundtrip) {
					t.Fatal("round trip", input, string(encoded), err)
				}
			}
			for _, field := range fields {
				if strings.Contains(" "+tc.allowed+" ", " "+field+" ") {
					continue
				}
				// Cross-variant fields must not escape validation through null or
				// zero values that the typed decoder would otherwise discard.
				for _, value := range []string{"null", "false", "[]"} {
					input := strings.TrimSuffix(tc.input, "}") + `,"` + field + `":` + value + "}"
					original := UnitType()
					got := original
					if err := json.Unmarshal([]byte(input), &got); err == nil {
						t.Fatal("accepted unrelated field", input)
					}
					if !reflect.DeepEqual(got, original) {
						t.Fatal("failed type decode changed receiver", input)
					}
				}
			}
		})
	}
}

func TestRowJSONVariantFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, allowed string }{
		{`{"kind":"empty"}`, ""},
		{`{"kind":"var","name":"r"}`, "name"},
		{`{"kind":"row","fields":[],"other":{"kind":"empty"}}`, "fields other"},
	} {
		var got Row
		if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
			t.Fatal(tc.input, err)
		}
		for _, field := range []string{"name", "fields", "other", "id", "elem", "value", "extra", "Kind", "Name", "Fields"} {
			if strings.Contains(" "+tc.allowed+" ", " "+field+" ") {
				continue
			}
			input := strings.TrimSuffix(tc.input, "}") + `,"` + field + `":null}`
			original := Row{Kind: "var", Name: "original"}
			got := original
			if err := json.Unmarshal([]byte(input), &got); err == nil {
				t.Fatal("accepted unrelated row field", input)
			}
			if !reflect.DeepEqual(got, original) {
				t.Fatal("failed row decode changed receiver", input)
			}
		}
	}
}

func TestTypeJSONNilArrayShapes(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", `,"args":null`, `,"args":[]`} {
		for _, tc := range []struct{ prefix, suffix string }{
			{`{"kind":"oper","res":{"kind":"bool"}`, "}"},
			{`{"kind":"app","ctor":{"kind":"const","name":"T"}`, "}"},
		} {
			input := tc.prefix + value + tc.suffix
			var got Type
			if err := json.Unmarshal([]byte(input), &got); err != nil {
				t.Fatal(input, err)
			}
			if (got.Args == nil) != (value != `,"args":[]`) {
				t.Fatal("changed absent/null/empty args", input)
			}
			encoded, err := json.Marshal(got)
			if err != nil || !strings.Contains(string(encoded), `"args":[]`) {
				t.Fatal("nil args no longer canonicalize to []", string(encoded), err)
			}
		}
	}
	for _, value := range []string{"", `,"fields":null`, `,"fields":[]`} {
		input := `{"kind":"row","other":{"kind":"empty"}` + value + "}"
		var got Row
		if err := json.Unmarshal([]byte(input), &got); err != nil {
			t.Fatal(input, err)
		}
		if (got.Fields == nil) != (value != `,"fields":[]`) {
			t.Fatal("changed absent/null/empty row fields", input)
		}
		encoded, err := json.Marshal(got)
		if err != nil || !strings.Contains(string(encoded), `"fields":[]`) {
			t.Fatal("nil fields no longer canonicalize to []", string(encoded), err)
		}
	}
}

func TestTypeJSONNestedSchemaFailures(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`{"kind":"set","elem":{"kind":"int","name":"extra"}}`,
		`{"kind":"list","elem":{"kind":"set","elem":{"kind":"bool","res":null}}}`,
		`{"kind":"fun","arg":{"kind":"int","extra":1},"res":{"kind":"bool"}}`,
		`{"kind":"fun","arg":{"kind":"int"},"res":{"kind":"bool","args":[]}}`,
		`{"kind":"oper","args":[{"kind":"var","name":"a","elem":null}],"res":{"kind":"bool"}}`,
		`{"kind":"oper","args":[],"res":{"kind":"int","other":null}}`,
		`{"kind":"app","args":[{"kind":"str","value":"extra"}],"ctor":{"kind":"const","name":"T"}}`,
		`{"kind":"app","args":[],"ctor":{"kind":"const","name":"T","res":null}}`,
		`{"kind":"tup","fields":{"kind":"row","fields":[],"other":{"kind":"empty","name":"extra"}}}`,
		`{"kind":"rec","fields":{"kind":"var","name":"r","fields":[]}}`,
		`{"kind":"sum","fields":{"kind":"row","fields":[{"fieldName":"A","fieldType":{"kind":"int","extra":1}}],"other":{"kind":"empty"}}}`,
		`{"kind":"rec","fields":{"kind":"row","fields":[{"fieldName":"a","fieldType":{"kind":"int"},"extra":null}],"other":{"kind":"empty"}}}`,
	} {
		original := UnitType()
		got := original
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Fatal("accepted nested schema violation", input)
		}
		if !reflect.DeepEqual(got, original) {
			t.Fatal("failed nested decode changed receiver", input)
		}
	}
}

func TestTypeJSONSemanticValidationUnchanged(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, want string }{
		{`{"kind":"const"}`, "const type missing name"},
		{`{"kind":"var","name":null}`, "var type missing name"},
		{`{"kind":"set","elem":null}`, "set type missing child"},
		{`{"kind":"list"}`, "list type missing child"},
		{`{"kind":"fun","arg":{"kind":"int"}}`, "fun type missing child"},
		{`{"kind":"oper","args":[]}`, "oper type missing child"},
		{`{"kind":"app","ctor":{"kind":"int"}}`, "type application requires constant constructor"},
		{`{"kind":"sum","fields":{"kind":"var","name":"r"}}`, "sum requires concrete row"},
		{`{"kind":"rec","fields":{"kind":"row","fields":[]}}`, "row missing tail"},
		{`{"kind":"rec","fields":{"kind":"var","name":""}}`, "row variable missing name"},
		{`{"kind":"int","id":-1}`, "negative IR ID -1"},
	} {
		var got Type
		if err := json.Unmarshal([]byte(tc.input), &got); err == nil || err.Error() != tc.want {
			t.Fatal(tc.input, err, "want", tc.want)
		}
	}
}

func TestRowFieldJSONSchemaAndPartialDecoding(t *testing.T) {
	t.Parallel()
	original := RowField{Name: "original", Type: Type{Kind: "list", Elem: &Type{Kind: "bool"}}}
	for _, input := range []string{
		`{"fieldName":"new","fieldType":{"kind":"str"},"extra":null}`,
		`{"fieldName":"new","fieldType":{"kind":"str"},"kind":"field"}`,
		`{"fieldName":"new","FieldType":{"kind":"str"}}`,
		`{"FieldName":"new","fieldType":{"kind":"str"}}`,
		`{"fieldName":"new","fieldType":{"kind":"int","args":[]}}`,
		`{"fieldName":"new","fieldType":null}`,
		`{"fieldName":false}`,
	} {
		got := original
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Fatal("accepted invalid row field", input)
		}
		if !reflect.DeepEqual(got, original) {
			t.Fatal("failed field decode changed receiver", input)
		}
	}
	for _, input := range []string{`null`, `{}`, `{"fieldName":null}`} {
		got := original
		if err := json.Unmarshal([]byte(input), &got); err != nil || !reflect.DeepEqual(got, original) {
			t.Fatal("changed partial/null field semantics", input, got, err)
		}
	}
	got := original
	if err := json.Unmarshal([]byte(`{"fieldName":"new"}`), &got); err != nil || got.Name != "new" || !reflect.DeepEqual(got.Type, original.Type) {
		t.Fatal("partial field name decode", got, err)
	}
	if err := json.Unmarshal([]byte(`{"fieldType":{"kind":"str"}}`), &got); err != nil || got.Name != "new" || got.Type.Kind != "str" {
		t.Fatal("partial field type decode", got, err)
	}
	var missing RowField
	if err := json.Unmarshal([]byte(`{}`), &missing); err != nil || !reflect.DeepEqual(missing, RowField{}) {
		t.Fatal("changed missing field semantics", missing, err)
	}
	if err := json.Unmarshal([]byte(`{"fieldName":"","fieldType":{"kind":"bool"}}`), &missing); err != nil || missing.Name != "" || missing.Type.Kind != "bool" {
		t.Fatal("empty field name is valid", missing, err)
	}
}

func TestTypeJSONMalformedObjectsPreserveReceivers(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`[]`, `1`, `"int"`, `true`, `{`, `{"kind":false}`} {
		originalType := UnitType()
		gotType := originalType
		if err := json.Unmarshal([]byte(input), &gotType); err == nil || !reflect.DeepEqual(gotType, originalType) {
			t.Fatal("invalid type object", input, gotType, err)
		}
		originalRow := Row{Kind: "var", Name: "original"}
		gotRow := originalRow
		if err := json.Unmarshal([]byte(input), &gotRow); err == nil || !reflect.DeepEqual(gotRow, originalRow) {
			t.Fatal("invalid row object", input, gotRow, err)
		}
	}
	var escaped Type
	if err := json.Unmarshal([]byte(`{"k\u0069nd":"set","e\u006cem":{"kind":"str"}}`), &escaped); err != nil || escaped.Kind != "set" || escaped.Elem.Kind != "str" {
		t.Fatal("valid escaped field names", escaped, err)
	}
}
