package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamMissingDeclarationTypeAbort(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source    string
		Modules         []Module
		Diagnostics     []Diagnostic
		Sources         SourceMap
		UpstreamSources UpstreamSourceMap
		NextID          ID
	}
	data, err := os.ReadFile("testdata/parser/parser_nil_type_abort.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 10 {
		t.Fatal("nil-type abort inventory", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			result, err := ParseModulesRecovering(fixture.Name+".qnt", fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			for _, snapshot := range []struct {
				name      string
				got, want any
			}{
				{"IR", result.Modules, fixture.Modules}, {"diagnostics", result.Diagnostics, fixture.Diagnostics},
				{"UTF16 spans", result.Sources, fixture.Sources}, {"raw spans", result.UpstreamSources, fixture.UpstreamSources},
			} {
				if !reflect.DeepEqual(snapshot.got, snapshot.want) {
					a, _ := json.Marshal(snapshot.got)
					b, _ := json.Marshal(snapshot.want)
					t.Errorf("%s\n got %s\nwant %s", snapshot.name, a, b)
				}
			}
			if got := result.IDs.Next(); got != fixture.NextID {
				t.Errorf("next ID got%s want%s", got, fixture.NextID)
			}
			if modules, sources, err := ParseModulesWithSource(fixture.Name+".qnt", fixture.Source); err == nil || modules != nil || sources != nil {
				t.Fatal("ordinary parser lost fail-fast behavior", err)
			}
		})
	}
}
