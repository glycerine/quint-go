package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamDiagnosticWireSchema(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Sources []struct {
			ID       ID
			Location Location
		}
		Cases []struct {
			Name                string
			Diagnostic, Located json.RawMessage
		}
	}
	data, err := os.ReadFile("testdata/parser/wire_diagnostic.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 16 {
		t.Fatal("missing diagnostic wire cases")
	}
	sources := SourceMap{}
	for _, source := range fixture.Sources {
		sources[source.ID] = source.Location
	}
	for _, f := range fixture.Cases {
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
			assertJSON := func(label string, value any, want json.RawMessage) {
				t.Helper()
				got, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decode(got), decode(want)) {
					t.Fatalf("%s Go %s\nTypeScript %s", label, got, want)
				}
			}
			var diagnostic Diagnostic
			if err := json.Unmarshal(f.Diagnostic, &diagnostic); err != nil {
				t.Fatal(err)
			}
			assertJSON("diagnostic roundtrip", diagnostic, f.Diagnostic)
			assertJSON("resolved diagnostic", ResolveDiagnostic(sources, diagnostic), f.Located)
			var located ErrorMessage
			if err := json.Unmarshal(f.Located, &located); err != nil {
				t.Fatal(err)
			}
			assertJSON("location roundtrip", located, f.Located)
		})
	}
}
