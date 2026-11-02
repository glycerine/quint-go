package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamSemanticParserRecovery(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source string
		Modules      json.RawMessage
		Diagnostics  []struct {
			Code, Message string
			Data          *DiagnosticData
			Span          *struct{ Start, End int }
		}
	}
	data, err := os.ReadFile("testdata/parser/parser_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte) any {
		t.Helper()
		var value any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return withoutParserIDs(value)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			result, err := ParseModulesRecovering(fixture.Name+".qnt", fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(result.Modules)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decode(got), decode(fixture.Modules)) {
				t.Fatalf("partial IR differs\nGo %s\nTS %s", got, fixture.Modules)
			}
			if len(result.Diagnostics) != len(fixture.Diagnostics) {
				t.Fatal(result.Diagnostics, fixture.Diagnostics)
			}
			for i, want := range fixture.Diagnostics {
				got := result.Diagnostics[i]
				if got.Code != want.Code || got.Message != want.Message || !reflect.DeepEqual(got.Data, want.Data) {
					t.Fatalf("diagnostic %d: %+v want %+v", i, got, want)
				}
				if got.Reference == nil || want.Span == nil {
					t.Fatal("missing diagnostic reference", got, want)
				}
				loc, ok := result.Sources[*got.Reference]
				if !ok || loc.Source != fixture.Name+".qnt" || loc.End == nil || loc.Start.Index != want.Span.Start || loc.End.Index != want.Span.End {
					t.Fatalf("diagnostic %d span: %+v want %+v", i, loc, want.Span)
				}
				if _, ok := result.UpstreamSources[*got.Reference]; !ok {
					t.Fatal("missing upstream location")
				}
			}
			if result.IDs == nil {
				t.Fatal("missing continuing generator")
			}
			next := result.IDs.Next()
			if _, exists := result.Sources[next]; exists {
				t.Fatal("continuing generator reused an allocated ID")
			}
			if len(fixture.Diagnostics) > 0 {
				modules, err := ParseModules(fixture.Source)
				var diagnostic Diagnostic
				if len(modules) != 0 || !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Diagnostics[0].Code {
					t.Fatal("ordinary entry point lost fail-fast behavior", modules, err)
				}
			}
		})
	}
}

func TestSemanticParserRecoveryFailuresAndIsolation(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"module M {", "module M { val x = @ }", string([]byte{0xff})} {
		result, err := ParseModulesRecovering("broken.qnt", source)
		if err == nil || len(result.Modules) != 0 {
			t.Fatal("accepted invalid syntax", source, result, err)
		}
		var syntax *SyntaxErrors
		if errors.As(err, &syntax) && syntax.Source != "broken.qnt" {
			t.Fatal("missing source name", syntax)
		}
	}
	const source = "module M { type lower = List[a] val x={a:1,a:2} }"
	first, err := ParseModulesRecovering("a.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseModulesRecovering("b.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	first.Diagnostics[0].Message = "changed"
	first.Modules[0].Declarations[0].Name = "changed"
	delete(first.Sources, *first.Diagnostics[0].Reference)
	if second.Diagnostics[0].Message == "changed" || second.Modules[0].Declarations[0].Name == "changed" || second.Sources[*second.Diagnostics[0].Reference].Source != "b.qnt" {
		t.Fatal("recovery results share mutable storage")
	}
}
