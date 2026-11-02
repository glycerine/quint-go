package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type typeCompositionSubstitution struct {
	Kind, Name string
	Value      json.RawMessage
}

func decodeTypeCompositionSubstitutions(t *testing.T, input []typeCompositionSubstitution) []Substitution {
	t.Helper()
	if input == nil {
		return nil
	}
	result := make([]Substitution, len(input))
	for i, value := range input {
		result[i] = Substitution{Kind: value.Kind, Name: value.Name}
		var err error
		if value.Kind == "type" {
			err = json.Unmarshal(value.Value, &result[i].Type)
		} else {
			err = json.Unmarshal(value.Value, &result[i].Row)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func TestUpstreamTypeCompositionFailuresAndPropagation(t *testing.T) {
	t.Parallel()
	var cases []struct {
		Name, Thrown, ConflictingName string
		First, Second, Substitutions  []typeCompositionSubstitution
		IncompatibleKinds             bool
		Error                         *ErrorTree
	}
	data, err := os.ReadFile("testdata/parser/type_composition.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 22 {
		t.Fatal("missing composition edge cases")
	}
	for _, f := range cases {
		t.Run(f.Name, func(t *testing.T) {
			first, second := decodeTypeCompositionSubstitutions(t, f.First), decodeTypeCompositionSubstitutions(t, f.Second)
			before, _ := json.Marshal([][]Substitution{first, second})
			got, failure := (TypeUnifier{}).Compose(first, second)
			if f.IncompatibleKinds {
				// Upstream throws a JavaScript exception containing the two
				// substitutions. Go returns its structured, stable cause.
				message := "Substitutions with same name (" + f.ConflictingName + ") but incompatible kinds"
				want := ErrorLeaf("composing substitutions", message)
				if failure == nil || !reflect.DeepEqual(*failure, want) || !strings.HasPrefix(f.Thrown, message+": [ ") {
					t.Fatal("incompatible substitution kinds", failure, f.Thrown)
				}
			} else if !reflect.DeepEqual(failure, f.Error) {
				t.Fatal("Go composition failure", failure, "TypeScript cause", f.Error)
			}
			if f.Thrown == "" {
				if !reflect.DeepEqual(got, decodeTypeCompositionSubstitutions(t, f.Substitutions)) {
					t.Fatalf("Go substitutions %+v\nTypeScript %+v", got, f.Substitutions)
				}
			} else {
				if got != nil || failure == nil {
					t.Fatal("composition failure retained partial substitutions", got, failure)
				}
				if !f.IncompatibleKinds && f.Thrown != "Unifying substitutions with same name: "+f.ConflictingName+", "+failure.String() {
					t.Fatal("composition exception lost its exact reference cause", f.Thrown, failure.String())
				}
			}
			after, _ := json.Marshal([][]Substitution{first, second})
			if string(before) != string(after) {
				t.Fatal("composition mutated input substitutions")
			}
		})
	}
}
