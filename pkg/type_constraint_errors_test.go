package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamTypeConstraintErrorAggregation(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name    string
		Input   []Module
		TypeIDs []string
		Errors  []struct {
			ID    string
			Error ErrorTree
		}
	}
	data, err := os.ReadFile("testdata/parser/type_constraint_errors.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(fixture.Input)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			result, err := NewTypeInferrer(linked.Table, nil).InferTypes(fixture.Input[0].Declarations)
			if err != nil {
				t.Fatal(err)
			}
			gotIDs := []string{}
			for id := range result.Types.All() {
				gotIDs = append(gotIDs, id)
			}
			if !reflect.DeepEqual(gotIDs, fixture.TypeIDs) {
				t.Fatal("type IDs", gotIDs, fixture.TypeIDs)
			}
			index := 0
			for id, failure := range result.Errors.All() {
				if index >= len(fixture.Errors) {
					t.Fatalf("unexpected error %s: %s", id, failure.String())
				}
				want := fixture.Errors[index]
				if id != want.ID || !reflect.DeepEqual(failure, want.Error) {
					got, _ := json.Marshal(failure)
					expected, _ := json.Marshal(want.Error)
					t.Errorf("error %s (want %s) Go %s\nTypeScript %s", id, want.ID, got, expected)
				}
				index++
			}
			if index != len(fixture.Errors) {
				t.Fatal("error count", index, len(fixture.Errors))
			}
		})
	}
}

func TestRecordConstraintErrorAggregation(t *testing.T) {
	id, _ := ParseID("1")
	result := Type{Kind: "var", Name: "result"}
	value := TypedExpression{Expr: Expr{Kind: "bool", Bool: true}, Type: Type{Kind: "bool"}}
	args := []TypedExpression{
		{Expr: Expr{Kind: "str", Str: "valid"}, Type: Type{Kind: "str"}}, value,
		{Expr: Expr{Kind: "bool", Bool: false}, Type: Type{Kind: "bool"}}, value,
		{Expr: Expr{Kind: "name", Name: "field"}, Type: Type{Kind: "str"}}, value,
	}
	constraints, failures := RecordConstructorConstraints(id, args, result)
	if constraints != nil || len(failures) != 2 {
		t.Fatal("expected all invalid fields without partial constraints", constraints, failures)
	}
	if *failures[0].Message != "Record field name must be a name expression but is bool: false" || *failures[1].Message != "Record field name must be a name expression but is name: field" {
		t.Fatal("field error order", failures)
	}
	constraints, combined := RecordConstraints("Rec", id, args, result)
	if constraints != nil || combined == nil || combined.Message != nil || !reflect.DeepEqual(combined.Children, failures) {
		t.Fatal("record convenience API dropped field failures", constraints, combined)
	}
	constraints, failures = RecordConstructorConstraints(id, args[:3], result)
	if constraints != nil || len(failures) != 1 || *failures[0].Message != "Record constructor requires field/value pairs" {
		t.Fatal("arity must precede field validation", constraints, failures)
	}
}
