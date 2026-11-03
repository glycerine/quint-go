package quint

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamRecordParserRecovery(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source    string
		Modules         json.RawMessage
		Diagnostics     []Diagnostic
		Sources         SourceMap
		UpstreamSources UpstreamSourceMap
		NextID          ID
		Partial         *struct {
			Modules         []Module
			Sources         SourceMap
			UpstreamSources UpstreamSourceMap
			NextID          ID
		}
	}
	data, err := os.ReadFile("testdata/parser/parser_record_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 28 {
		t.Fatal("record recovery inventory", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			result, err := ParseModulesRecovering(fixture.Name+".qnt", fixture.Source)
			var modules []Module
			sources, upstream, next := fixture.Sources, fixture.UpstreamSources, fixture.NextID
			if fixture.Partial != nil {
				var unsupported *SyntaxRecoveryError
				if !errors.As(err, &unsupported) {
					t.Fatal("null reference operand lost explicit unsupported boundary", err)
				}
				modules = fixture.Partial.Modules
				sources, upstream, next = fixture.Partial.Sources, fixture.Partial.UpstreamSources, fixture.Partial.NextID
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(fixture.Modules, &modules); err != nil {
					t.Fatal(err)
				}
			}
			for _, comparison := range []struct {
				name      string
				got, want any
			}{{"raw IR", result.Modules, modules}, {"diagnostics", result.Diagnostics, fixture.Diagnostics}, {"UTF16 spans", result.Sources, sources}, {"raw spans", result.UpstreamSources, upstream}} {
				if !reflect.DeepEqual(comparison.got, comparison.want) {
					a, _ := json.Marshal(comparison.got)
					b, _ := json.Marshal(comparison.want)
					t.Errorf("%s\n got %s\nwant %s", comparison.name, a, b)
				}
			}
			if got := result.IDs.Next(); got != next {
				t.Errorf("next ID got%s want%s", got, next)
			}
			plain, spans, plainErr := ParseModulesWithSource(fixture.Name+".qnt", fixture.Source)
			if len(fixture.Diagnostics) > 0 {
				if plainErr == nil || plain != nil || spans != nil {
					t.Fatal("ordinary parser lost fail-fast behavior", plainErr)
				}
			} else if plainErr != nil || !reflect.DeepEqual(plain, modules) || !reflect.DeepEqual(spans, sources) {
				t.Fatal("healthy record parsing changed", plainErr)
			}
		})
	}
}
