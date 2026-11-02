package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamSyntaxParserRecovery(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source string
		Modules      json.RawMessage
		Diagnostics  []struct {
			Diagnostic
			Location UpstreamLocation
		}
	}
	data, err := os.ReadFile("testdata/parser/parser_syntax_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte) any {
		var value any
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if err := d.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return withoutParserIDs(value)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			result, err := ParseModulesRecovering(f.Name+".qnt", f.Source)
			if err != nil {
				t.Fatalf("%v; diagnostics %+v", err, result.Diagnostics)
			}
			actual, err := json.Marshal(result.Modules)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decode(actual), decode(f.Modules)) {
				t.Errorf("Go %s\nTS %s", actual, f.Modules)
			}
			if len(result.Diagnostics) != len(f.Diagnostics) {
				t.Fatalf("Go %+v\nTS %+v", result.Diagnostics, f.Diagnostics)
			}
			for i, want := range f.Diagnostics {
				got := result.Diagnostics[i]
				if got.Code != want.Code || got.Message != want.Message || !reflect.DeepEqual(got.Data, want.Data) {
					t.Errorf("diag%d Go%+v TS%+v", i, got, want)
				}
				if got.Reference == nil {
					t.Fatal("missing syntax diagnostic reference", got)
				}
				if !reflect.DeepEqual(result.UpstreamSources[*got.Reference], want.Location) {
					t.Errorf("span%d Go%+v TS%+v", i, result.UpstreamSources[*got.Reference], want.Location)
				}
				location, ok := result.Sources[*got.Reference]
				if !ok || location.Source != f.Name+".qnt" || location.End == nil || location.Start.Index > location.End.Index {
					t.Fatal("invalid public syntax location", location)
				}
				file, _ := NewSourceFile(f.Name+".qnt", f.Source)
				for _, position := range []Position{location.Start, *location.End} {
					byteOffset, err := file.ByteOffset(position.Line, position.Col)
					if err != nil {
						t.Fatal(err)
					}
					actual, _ := file.PositionAt(byteOffset)
					if actual != position {
						t.Fatal("inconsistent UTF-16 syntax location", position, actual)
					}
				}
			}
		})
	}
}

func TestUnsupportedSyntaxRecoveryBoundary(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"module Good { val x=1 } module Bad { val r={a:1,b:val y=2} }",
	} {
		result, err := ParseModulesRecovering("unsupported.qnt", source)
		var unsupported *SyntaxRecoveryError
		if !errors.As(err, &unsupported) || len(result.Diagnostics) == 0 || len(result.Modules) != 1 || result.Modules[0].Name != "Good" {
			t.Fatal("unsupported malformed tree lost explicit boundary or completed module", result, err)
		}
		if unsupported.Unwrap() == nil {
			t.Fatal("missing recovery cause")
		}
	}
}
