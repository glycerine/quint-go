package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamNondetChecker(t *testing.T) {
	var fixtures []struct {
		Name  string
		Input []Module
		Types []struct {
			ID     string
			Scheme TypeScheme
		}
		Errors []Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/nondet.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			types := insdict.NewDict[string, TypeScheme]()
			for _, entry := range f.Types {
				types.Set(entry.ID, entry.Scheme)
			}
			got, err := NewNondetChecker(linked.Table).Check(types, f.Input[0].Declarations)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, f.Errors) {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(f.Errors)
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
}
