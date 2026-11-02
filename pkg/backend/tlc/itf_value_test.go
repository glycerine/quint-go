package tlc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	native "github.com/glycerine/tlago/tlc"
)

func itfTestRow(kind string, fields ...quint.RowField) quint.Type {
	return quint.Type{Kind: kind, Fields: &quint.Row{Kind: "row", Fields: fields, Other: &quint.Row{Kind: "empty"}}}
}

func itfTestTuple(types ...quint.Type) quint.Type {
	fields := make([]quint.RowField, len(types))
	for i, typ := range types {
		fields[i] = quint.RowField{Name: strconv.Itoa(i), Type: typ}
	}
	return itfTestRow("tup", fields...)
}

func itfTestRecord(names []string, values ...native.Value) *native.RecordValue {
	keys := make([]*native.UniqueString, len(names))
	for i, name := range names {
		keys[i] = native.UniqueStringOf(name)
	}
	return native.NewRecordValue(keys, values, false)
}

func assertNativeITF(t *testing.T, value native.Value, typ quint.Type, want string) {
	t.Helper()
	expression, err := nativeITFExpr(context.Background(), value, typ)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := quint.EncodeITFValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	var gotJSON, wantJSON any
	if err := json.Unmarshal(encoded, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Fatal("invalid expected JSON", err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("ITF = %s; want %s", encoded, want)
	}
}

func TestNativeITFPrimitivesAndCollections(t *testing.T) {
	integer, boolean, stringType := quint.Type{Kind: "int"}, quint.Type{Kind: "bool"}, quint.Type{Kind: "str"}
	setType := quint.Type{Kind: "set", Elem: &integer}
	listType := quint.Type{Kind: "list", Elem: &integer}
	pairType := itfTestTuple(integer, integer)
	intMap := quint.Type{Kind: "fun", Arg: &integer, Res: &integer}
	stringMap := quint.Type{Kind: "fun", Arg: &stringType, Res: &integer}
	stringRecord := itfTestRow("rec", quint.RowField{Name: "a", Type: integer}, quint.RowField{Name: "b", Type: integer})
	stringKeys := native.NewFcnRcdValue([]native.Value{native.NewStringValue("a"), native.NewStringValue("b")}, []native.Value{native.NewIntValue(1), native.NewIntValue(2)}, true)
	contiguousKeys := native.NewFcnRcdIntervalValue(native.NewIntervalValue(1, 2), []native.Value{native.NewIntValue(4), native.NewIntValue(5)})
	for _, tc := range []struct {
		name  string
		value native.Value
		typ   quint.Type
		want  string
	}{
		{"false", native.NewBoolValue(false), boolean, `false`},
		{"true", native.NewBoolValue(true), boolean, `true`},
		{"zero", native.NewIntValue(0), integer, `{"#bigint":"0"}`},
		{"minimum integer", native.NewIntValue(-2147483648), integer, `{"#bigint":"-2147483648"}`},
		{"maximum integer", native.NewIntValue(2147483647), integer, `{"#bigint":"2147483647"}`},
		{"escaped unicode string", native.NewStringValue("\"\\\n\r\t\x00é𝄞"), stringType, `"\"\\\n\r\t\u0000é𝄞"`},
		{"empty string", native.NewStringValue(""), stringType, `""`},
		{"set", native.NewSetEnumValue([]native.Value{native.NewIntValue(2), native.NewIntValue(1), native.NewIntValue(2)}, false), setType, `{"#set":[{"#bigint":"1"},{"#bigint":"2"}]}`},
		{"interval set", native.NewIntervalValue(-1, 1), setType, `{"#set":[{"#bigint":"-1"},{"#bigint":"0"},{"#bigint":"1"}]}`},
		{"empty interval set", native.NewIntervalValue(2, 1), setType, `{"#set":[]}`},
		{"tuple", native.NewTupleValue([]native.Value{native.NewIntValue(4), native.NewIntValue(5)}), pairType, `{"#tup":[{"#bigint":"4"},{"#bigint":"5"}]}`},
		{"list", native.NewTupleValue([]native.Value{native.NewIntValue(4), native.NewIntValue(5)}), listType, `[{"#bigint":"4"},{"#bigint":"5"}]`},
		{"string keyed map", stringKeys, stringMap, `{"#map":[["a",{"#bigint":"1"}],["b",{"#bigint":"2"}]]}`},
		{"string keyed function as record", stringKeys, stringRecord, `{"a":{"#bigint":"1"},"b":{"#bigint":"2"}}`},
		{"record as string keyed map", itfTestRecord([]string{"a", "b"}, native.NewIntValue(1), native.NewIntValue(2)), stringMap, `{"#map":[["a",{"#bigint":"1"}],["b",{"#bigint":"2"}]]}`},
		{"contiguous integer map", contiguousKeys, intMap, `{"#map":[[{"#bigint":"1"},{"#bigint":"4"}],[{"#bigint":"2"},{"#bigint":"5"}]]}`},
		{"contiguous integer function as list", contiguousKeys, listType, `[{"#bigint":"4"},{"#bigint":"5"}]`},
		{"contiguous integer function as tuple", contiguousKeys, pairType, `{"#tup":[{"#bigint":"4"},{"#bigint":"5"}]}`},
		{"tuple as integer map", native.NewTupleValue([]native.Value{native.NewIntValue(4), native.NewIntValue(5)}), intMap, `{"#map":[[{"#bigint":"1"},{"#bigint":"4"}],[{"#bigint":"2"},{"#bigint":"5"}]]}`},
		{"empty map", native.NewFcnRcdValue([]native.Value{}, []native.Value{}, true), intMap, `{"#map":[]}`},
		{"empty list", native.NewTupleValue([]native.Value{}), listType, `[]`},
		{"empty tuple", native.NewTupleValue([]native.Value{}), quint.UnitType(), `{"#tup":[]}`},
		{"generated unit", itfTestRecord([]string{"tag"}, native.NewStringValue("UNIT")), quint.UnitType(), `{"#tup":[]}`},
		{"empty record", itfTestRecord([]string{}), itfTestRow("rec"), `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) { assertNativeITF(t, tc.value, tc.typ, tc.want) })
	}
}

func TestNativeITFVariantsAndNestedValues(t *testing.T) {
	integer, stringType := quint.Type{Kind: "int"}, quint.Type{Kind: "str"}
	variantType := itfTestRow("sum", quint.RowField{Name: "Some", Type: integer}, quint.RowField{Name: "None", Type: quint.UnitType()})
	assertNativeITF(t, native.NewFcnRcdValue([]native.Value{native.NewStringValue("Some")}, []native.Value{native.NewIntValue(7)}, true), variantType, `{"tag":"Some","value":{"#bigint":"7"}}`)
	assertNativeITF(t, itfTestRecord([]string{"None"}, itfTestRecord([]string{"tag"}, native.NewStringValue("UNIT"))), variantType, `{"tag":"None","value":{"#tup":[]}}`)

	pairType := itfTestTuple(integer, stringType)
	setType := quint.Type{Kind: "set", Elem: &pairType}
	mapType := quint.Type{Kind: "fun", Arg: &stringType, Res: &setType}
	listType := quint.Type{Kind: "list", Elem: &variantType}
	recordType := itfTestRow("rec", quint.RowField{Name: "lookup", Type: mapType}, quint.RowField{Name: "items", Type: listType})
	value := itfTestRecord([]string{"lookup", "items"},
		native.NewFcnRcdValue([]native.Value{native.NewStringValue("key")}, []native.Value{native.NewSetEnumValue([]native.Value{native.NewTupleValue([]native.Value{native.NewIntValue(-3), native.NewStringValue("é")})}, true)}, true),
		native.NewTupleValue([]native.Value{native.NewFcnRcdValue([]native.Value{native.NewStringValue("Some")}, []native.Value{native.NewIntValue(8)}, true)}),
	)
	assertNativeITF(t, value, recordType, `{"lookup":{"#map":[["key",{"#set":[{"#tup":[{"#bigint":"-3"},"é"]}]}]]},"items":[{"tag":"Some","value":{"#bigint":"8"}}]}`)

	// Tuple map keys must remain structured ITF values rather than JSON object
	// property names, including when their components are themselves strings.
	assertNativeITF(t, native.NewFcnRcdValue([]native.Value{native.NewTupleValue([]native.Value{native.NewIntValue(1), native.NewStringValue("a")})}, []native.Value{native.NewIntValue(9)}, true), quint.Type{Kind: "fun", Arg: &pairType, Res: &integer}, `{"#map":[[{"#tup":[{"#bigint":"1"},"a"]},{"#bigint":"9"}]]}`)
}

func TestNativeITFClosedRowChain(t *testing.T) {
	integer, boolean := quint.Type{Kind: "int"}, quint.Type{Kind: "bool"}
	// Solved type rows may retain a closed row tail rather than putting every
	// field in the first row node.
	typ := quint.Type{Kind: "rec", Fields: &quint.Row{Kind: "row", Fields: []quint.RowField{{Name: "x", Type: integer}}, Other: &quint.Row{Kind: "row", Fields: []quint.RowField{{Name: "y", Type: boolean}}, Other: &quint.Row{Kind: "empty"}}}}
	assertNativeITF(t, itfTestRecord([]string{"x", "y"}, native.NewIntValue(1), native.NewBoolValue(true)), typ, `{"x":{"#bigint":"1"},"y":true}`)
}

func TestNativeITFRejectsMismatchedValues(t *testing.T) {
	integer, boolean, stringType := quint.Type{Kind: "int"}, quint.Type{Kind: "bool"}, quint.Type{Kind: "str"}
	setType, listType := quint.Type{Kind: "set", Elem: &integer}, quint.Type{Kind: "list", Elem: &integer}
	mapType := quint.Type{Kind: "fun", Arg: &integer, Res: &integer}
	variantType := itfTestRow("sum", quint.RowField{Name: "Some", Type: integer})
	var nilInteger *native.IntValue
	var nilString *native.StringValue
	for _, tc := range []struct {
		name  string
		value native.Value
		typ   quint.Type
	}{
		{"nil value", nil, integer},
		{"typed nil value", nilInteger, integer},
		{"typed nil string", nilString, stringType},
		{"integer mismatch", native.NewBoolValue(true), integer},
		{"boolean mismatch", native.NewStringValue("TRUE"), boolean},
		{"string mismatch", native.NewIntValue(1), stringType},
		{"set mismatch", native.NewIntValue(1), setType},
		{"set element mismatch", native.NewSetEnumValue([]native.Value{native.NewStringValue("1")}, true), setType},
		{"list mismatch", native.NewSetEnumValue([]native.Value{native.NewIntValue(1)}, true), listType},
		{"list element mismatch", native.NewTupleValue([]native.Value{native.NewBoolValue(true)}), listType},
		{"tuple arity mismatch", native.NewTupleValue([]native.Value{native.NewIntValue(1)}), itfTestTuple(integer, integer)},
		{"tuple element mismatch", native.NewTupleValue([]native.Value{native.NewBoolValue(true)}), itfTestTuple(integer)},
		{"record missing field", itfTestRecord([]string{}), itfTestRow("rec", quint.RowField{Name: "x", Type: integer})},
		{"record extra field", itfTestRecord([]string{"x"}, native.NewIntValue(1)), itfTestRow("rec")},
		{"map key mismatch", native.NewFcnRcdValue([]native.Value{native.NewStringValue("x")}, []native.Value{native.NewIntValue(1)}, true), mapType},
		{"map result mismatch", native.NewFcnRcdValue([]native.Value{native.NewIntValue(1)}, []native.Value{native.NewStringValue("x")}, true), mapType},
		{"list noncontiguous domain", native.NewFcnRcdValue([]native.Value{native.NewIntValue(2)}, []native.Value{native.NewIntValue(1)}, true), listType},
		{"unknown variant", itfTestRecord([]string{"Other"}, native.NewIntValue(1)), variantType},
		{"variant with multiple tags", itfTestRecord([]string{"Some", "Other"}, native.NewIntValue(1), native.NewIntValue(2)), variantType},
		{"variant payload mismatch", itfTestRecord([]string{"Some"}, native.NewBoolValue(true)), variantType},
		{"unit tag mismatch", itfTestRecord([]string{"tag"}, native.NewStringValue("OTHER")), quint.UnitType()},
		{"unknown type", native.NewIntValue(1), quint.Type{Kind: "mystery"}},
		{"unresolved type variable", native.NewIntValue(1), quint.Type{Kind: "var", Name: "a"}},
		{"missing set element type", native.NewSetEnumValue([]native.Value{}, true), quint.Type{Kind: "set"}},
		{"missing list element type", native.NewTupleValue([]native.Value{}), quint.Type{Kind: "list"}},
		{"missing map argument type", native.NewFcnRcdValue([]native.Value{}, []native.Value{}, true), quint.Type{Kind: "fun", Res: &integer}},
		{"missing record row", itfTestRecord([]string{}), quint.Type{Kind: "rec"}},
		{"open record row", itfTestRecord([]string{}), quint.Type{Kind: "rec", Fields: &quint.Row{Kind: "var", Name: "r"}}},
		{"missing record row tail", itfTestRecord([]string{}), quint.Type{Kind: "rec", Fields: &quint.Row{Kind: "row", Fields: []quint.RowField{}}}},
		{"duplicate record type field", itfTestRecord([]string{"x", "y"}, native.NewIntValue(1), native.NewIntValue(2)), itfTestRow("rec", quint.RowField{Name: "x", Type: integer}, quint.RowField{Name: "x", Type: integer})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if expression, err := nativeITFExpr(context.Background(), tc.value, tc.typ); err == nil {
				t.Fatalf("accepted mismatched value as %+v", expression)
			}
		})
	}
}

func TestNativeITFCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nativeITFExpr(ctx, native.NewIntValue(1), quint.Type{Kind: "int"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled conversion returned %v", err)
	}
}

func TestNativeITFTupleNumericRowOrder(t *testing.T) {
	typ := itfTestRow("tup", quint.RowField{Name: "1", Type: quint.Type{Kind: "str"}}, quint.RowField{Name: "0", Type: quint.Type{Kind: "int"}})
	value := native.NewTupleValue([]native.Value{native.NewIntValue(3), native.NewStringValue("x")})
	assertNativeITF(t, value, typ, `{"#tup":[{"#bigint":"3"},"x"]}`)
	for _, name := range []string{"a", "-1", "+1", "01"} {
		typ := itfTestRow("tup", quint.RowField{Name: name, Type: quint.Type{Kind: "int"}})
		if _, err := nativeITFExpr(context.Background(), native.NewTupleValue([]native.Value{native.NewIntValue(3)}), typ); err == nil {
			t.Fatal("accepted invalid tuple position", name)
		}
	}
}
