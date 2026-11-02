package quint

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestUpstreamLocalResolution(t *testing.T) {
	type binding struct {
		Reference, Definition string
		Depth                 int
	}
	type diagnostic struct{ Code, Message, Reference string }
	var fixtures []struct {
		Name, Source string
		Bindings     []binding
		Errors       []diagnostic
	}
	data, err := os.ReadFile("testdata/parser/resolution.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			ms, err := ParseModules(f.Source)
			if err != nil {
				t.Fatal(err)
			}
			paths := map[ID]string{}
			data, _ := json.Marshal(ms)
			var tree any
			if err = json.Unmarshal(data, &tree); err != nil {
				t.Fatal(err)
			}
			var visit func(any, string)
			visit = func(v any, path string) {
				switch x := v.(type) {
				case map[string]any:
					if n, ok := x["id"].(float64); ok {
						id, _ := ParseID(fmt.Sprint(n))
						if _, ok := paths[id]; !ok {
							paths[id] = path
						}
					}
					keys := []string{}
					for k := range x {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						if k != "id" {
							visit(x[k], path+"/"+k)
						}
					}
				case []any:
					for i, child := range x {
						visit(child, fmt.Sprintf("%s/%d", path, i))
					}
				}
			}
			visit(tree, "")
			r, err := ResolveLocalModule(ms[0])
			if err != nil {
				t.Fatal(err)
			}
			got := []binding{}
			for id, def := range r.Table.All() {
				got = append(got, binding{paths[id], paths[def.ID], def.Depth})
			}
			if !reflect.DeepEqual(got, f.Bindings) {
				t.Errorf("bindings Go %+v\nTS %+v", got, f.Bindings)
			}
			ds := []diagnostic{}
			for _, d := range r.Errors {
				reference := ""
				if d.Reference != nil {
					reference = paths[*d.Reference]
				}
				ds = append(ds, diagnostic{d.Code, d.Message, reference})
			}
			if !reflect.DeepEqual(ds, f.Errors) {
				t.Errorf("diagnostics Go %+v\nTS %+v", ds, f.Errors)
			}
		})
	}
}
