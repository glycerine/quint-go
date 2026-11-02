package quint

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamSourceSpanFixtures(t *testing.T) {
	type span struct{ Start, End int }
	var fixtures []struct {
		Name, Source string
		Spans        map[string]span
	}
	data, err := os.ReadFile("testdata/parser/spans.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			modules, sources, err := ParseModulesWithSource(f.Name, f.Source)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(modules)
			if err != nil {
				t.Fatal(err)
			}
			var tree any
			if err = json.Unmarshal(data, &tree); err != nil {
				t.Fatal(err)
			}
			got := map[string]span{}
			var visit func(any, string)
			visit = func(v any, path string) {
				switch x := v.(type) {
				case map[string]any:
					if idNumber, ok := x["id"].(float64); ok {
						id, _ := ParseID(fmt.Sprint(idNumber))
						loc, ok := sources[id]
						if !ok || loc.End == nil {
							t.Fatalf("missing span %s", path)
						}
						got[path] = span{loc.Start.Index, loc.End.Index}
					}
					for k, child := range x {
						if k != "id" {
							visit(child, path+"/"+k)
						}
					}
				case []any:
					for i, child := range x {
						visit(child, fmt.Sprintf("%s/%d", path, i))
					}
				}
			}
			visit(tree, "")
			if !reflect.DeepEqual(got, f.Spans) {
				for path, want := range f.Spans {
					if got[path] != want {
						t.Errorf("%s: Go %+v, upstream %+v", path, got[path], want)
					}
				}
				for path := range got {
					if _, ok := f.Spans[path]; !ok {
						t.Errorf("extra source span %s", path)
					}
				}
			}
		})
	}
}
