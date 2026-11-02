package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTypeNames(t *testing.T) {
	var typ Type
	data := `{"kind":"oper","args":[{"kind":"var","name":"a"},{"kind":"rec","fields":{"kind":"row","fields":[{"fieldName":"nested","fieldType":{"kind":"sum","fields":{"kind":"row","fields":[{"fieldName":"Some","fieldType":{"kind":"var","name":"b"}}],"other":{"kind":"var","name":"inner"}}}}],"other":{"kind":"var","name":"a"}}}],"res":{"kind":"app","ctor":{"kind":"const","name":"Box"},"args":[{"kind":"var","name":"a"}]}}`
	if err := json.Unmarshal([]byte(data), &typ); err != nil {
		t.Fatal(err)
	}
	types, rows, err := TypeNames(&typ)
	if err != nil || !reflect.DeepEqual(types, []string{"a", "b"}) || !reflect.DeepEqual(rows, []string{"inner", "a"}) {
		t.Fatalf("names: %v %v %v", types, rows, err)
	}
	if _, _, err = TypeNames(&Type{Kind: "unknown"}); err == nil {
		t.Fatal("accepted unknown type")
	}
	for _, typ := range []*Type{nil, {Kind: "int"}, {Kind: "const", Name: "A"}} {
		types, rows, err := TypeNames(typ)
		if err != nil || len(types) != 0 || len(rows) != 0 {
			t.Fatalf("unexpected names %v %v %v", types, rows, err)
		}
	}
}
func TestRowNamesAndFields(t *testing.T) {
	row := &Row{Kind: "row", Fields: []RowField{{Name: "first", Type: Type{Kind: "var", Name: "fieldVar"}}}, Other: &Row{Kind: "row", Fields: []RowField{{Name: "second", Type: Type{Kind: "int"}}}, Other: &Row{Kind: "var", Name: "tail"}}}
	if got := RowFieldNames(row); !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatal(got)
	}
	if got := RowNames(row); !reflect.DeepEqual(got, []string{"tail"}) {
		t.Fatal(got)
	}
	for _, row := range []*Row{nil, {Kind: "empty"}} {
		if len(RowNames(row)) != 0 || len(RowFieldNames(row)) != 0 {
			t.Fatal("nonempty result")
		}
	}
}
