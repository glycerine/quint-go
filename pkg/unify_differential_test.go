package quint

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamUnification(t *testing.T) {
	var fixtures []struct {
		Left, Right   Type
		Substitutions []struct {
			Kind, Name string
			Value      json.RawMessage
		}
		Error *ErrorTree
	}
	data, err := os.ReadFile("testdata/parser/unify.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for i, f := range fixtures {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			subs, e := (TypeUnifier{}).Unify(f.Left, f.Right)
			if !reflect.DeepEqual(e, f.Error) {
				t.Fatalf("Go %+v TS %+v", e, f.Error)
			}
			if e != nil {
				return
			}
			want := []Substitution{}
			for _, s := range f.Substitutions {
				v := Substitution{Kind: s.Kind, Name: s.Name}
				if s.Kind == "type" {
					err = json.Unmarshal(s.Value, &v.Type)
				} else {
					err = json.Unmarshal(s.Value, &v.Row)
				}
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, v)
			}
			if !reflect.DeepEqual(subs, want) {
				a, _ := json.Marshal(subs)
				b, _ := json.Marshal(want)
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
}
