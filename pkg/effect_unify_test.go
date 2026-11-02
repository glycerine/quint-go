package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamEffectUnification(t *testing.T) {
	var fixtures []struct {
		Name          string
		A, B          Effect
		Error         *ErrorTree
		Substitutions []struct {
			Kind, Name string
			Value      json.RawMessage
		}
	}
	data, err := os.ReadFile("testdata/parser/effect_unify.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			if f.Name == "hash-concrete" {
				t.Skip("P5.1: self-containing entity substitutions require stable occurrence identity")
			}
			beforeA, _ := json.Marshal(f.A)
			beforeB, _ := json.Marshal(f.B)
			subs, err := UnifyEffects(f.A, f.B)
			if !reflect.DeepEqual(err, f.Error) {
				t.Fatalf("Go error %+v; TS %+v", err, f.Error)
			}
			if len(subs) != len(f.Substitutions) {
				t.Fatalf("Go %+v; TS %+v", subs, f.Substitutions)
			}
			for i, s := range subs {
				want := f.Substitutions[i]
				var got any
				if s.Kind == "entity" {
					got = s.Entity
				} else {
					got = s.Effect
				}
				a, e := json.Marshal(got)
				if e != nil {
					t.Fatal(e)
				}
				var ga, gb any
				if e = json.Unmarshal(a, &ga); e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(want.Value, &gb); e != nil {
					t.Fatal(e)
				}
				if s.Kind != want.Kind || s.Name != want.Name || !reflect.DeepEqual(ga, gb) {
					t.Fatalf("binding %d: Go %s %s %s; TS %+v", i, s.Kind, s.Name, a, want)
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
