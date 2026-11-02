package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

// The default comparison never materializes a million-field tuple. The actual
// upstream probe saves only its boundary fields and count; Go must reject that
// allocation while retaining the exact ordinary and MAX_SAFE_INTEGER behavior.
func TestUpstreamTuplePrefixConstraintsAndResourcePolicy(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Cases []struct {
			Name, Index, ResourcePolicy string
			ID                          ID
			Args                        []TypedExpression
			Result                      Type
			Constraints                 []Constraint
			GoError                     *ErrorTree
			ReferenceSummary            struct {
				FieldCount int
				Samples    []struct {
					Position int
					Field    RowField
				}
				Tail Row
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/type_tuple_prefix.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 11 {
		t.Fatal("missing tuple boundary cases")
	}
	for _, f := range fixture.Cases {
		t.Run(f.Name, func(t *testing.T) {
			before, _ := json.Marshal(f.Args)
			constraints, failure := TupleItemConstraints(f.ID, f.Args, f.Result)
			if !reflect.DeepEqual(failure, f.GoError) {
				t.Fatal("Go error", failure, "expected", f.GoError)
			}
			if f.GoError == nil {
				if !reflect.DeepEqual(constraints, f.Constraints) {
					got, _ := json.Marshal(constraints)
					want, _ := json.Marshal(f.Constraints)
					t.Fatalf("Go constraints %s\nTypeScript %s", got, want)
				}
			} else {
				if constraints != nil || f.ResourcePolicy == "" || f.ReferenceSummary.FieldCount != 1000002 || f.ReferenceSummary.Tail.Kind != "var" || f.ReferenceSummary.Tail.Name != "tail_result" {
					t.Fatal("missing actual reference allocation evidence")
				}
				for _, sample := range f.ReferenceSummary.Samples {
					position := strconv.Itoa(sample.Position)
					want := RowField{Name: position, Type: Type{Kind: "var", Name: "tup_result_" + position}}
					if sample.Position == f.ReferenceSummary.FieldCount-1 {
						want.Type = f.Result
					}
					if sample.Position < 0 || sample.Position >= f.ReferenceSummary.FieldCount || !reflect.DeepEqual(sample.Field, want) {
						t.Fatal("invalid reference prefix sample")
					}
				}
			}
			after, _ := json.Marshal(f.Args)
			if string(before) != string(after) {
				t.Fatal("tuple constraint generation mutated its arguments")
			}
		})
	}
}
