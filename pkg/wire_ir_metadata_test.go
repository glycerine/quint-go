package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamRawIRMetadataWireSchema(t *testing.T) {
	t.Parallel()
	var cases []struct {
		Name   string
		Target string
		Input  []Module
		Output json.RawMessage
	}
	data, err := os.ReadFile("testdata/parser/wire_ir_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 10 {
		t.Fatal("missing raw IR wire cases")
	}
	for _, f := range cases {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			decode := func(raw []byte) any {
				t.Helper()
				decoder := json.NewDecoder(bytes.NewReader(raw))
				decoder.UseNumber()
				var value any
				if err := decoder.Decode(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			assertRaw := func(label string, module Module) {
				t.Helper()
				encoded, err := json.Marshal(module)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decode(encoded), decode(f.Output)) {
					t.Fatalf("%s Go %s\nTypeScript %s", label, encoded, f.Output)
				}
			}
			var roundtrip Module
			if err := json.Unmarshal(f.Output, &roundtrip); err != nil {
				t.Fatal(err)
			}
			assertRaw("raw IR roundtrip", roundtrip)
			linked, err := ResolveLinkedModules(f.Input)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			modules := insdict.NewDict[string, Module]()
			for _, module := range f.Input {
				modules.Set(module.Name, module)
			}
			out, err := FlattenModule(modules.Get(f.Target), modules, linked.Table)
			if err != nil {
				t.Fatal(err)
			}
			assertRaw("flattened IR", out)
		})
	}
}
