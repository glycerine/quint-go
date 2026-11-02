package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamEntityUnification(t *testing.T) {
	var fixtures []struct {
		Name          string
		A, B          Entity
		Error         *ErrorTree
		Substitutions []struct {
			Kind, Name string
			Value      Entity
		}
	}
	data, err := os.ReadFile("testdata/parser/entity_unify.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			beforeA, _ := json.Marshal(f.A)
			beforeB, _ := json.Marshal(f.B)
			subs, err := UnifyEntities(f.A, f.B)
			if !reflect.DeepEqual(err, f.Error) {
				t.Fatalf("Go error %+v; TS %+v", err, f.Error)
			}
			if len(subs) != len(f.Substitutions) {
				t.Fatalf("got %+v; want %+v", subs, f.Substitutions)
			}
			for i, s := range subs {
				want := f.Substitutions[i]
				a, _ := json.Marshal(s.Entity)
				b, _ := json.Marshal(want.Value)
				if s.Kind != want.Kind || s.Name != want.Name || string(a) != string(b) {
					t.Fatalf("binding %d: %+v %s; want %+v", i, s, a, want)
				}
				if len(s.Entity.StateVariables) > 0 {
					s.Entity.StateVariables[0].Name = "mutated"
				}
			}
			afterA, _ := json.Marshal(f.A)
			afterB, _ := json.Marshal(f.B)
			if string(beforeA) != string(afterA) || string(beforeB) != string(afterB) {
				t.Fatal("input mutated")
			}
		})
	}
}
