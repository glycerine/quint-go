package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamRecoveryExpressionStack(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source    string
		Modules         []Module
		Diagnostics     []Diagnostic
		Sources         SourceMap
		UpstreamSources UpstreamSourceMap
		NextID          ID
	}
	data, err := os.ReadFile("testdata/parser/parser_recovery_stack.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 26 {
		t.Fatal("recovery stack inventory", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseModulesRecovering(fixture.Name+".qnt", fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Modules, fixture.Modules) {
				a, _ := json.Marshal(got.Modules)
				b, _ := json.Marshal(fixture.Modules)
				t.Errorf("IR\n got %s\nwant %s", a, b)
			}
			if !reflect.DeepEqual(got.Diagnostics, fixture.Diagnostics) {
				a, _ := json.Marshal(got.Diagnostics)
				b, _ := json.Marshal(fixture.Diagnostics)
				t.Errorf("diagnostics\n got %s\nwant %s", a, b)
			}
			if !reflect.DeepEqual(got.Sources, fixture.Sources) {
				a, _ := json.Marshal(got.Sources)
				b, _ := json.Marshal(fixture.Sources)
				t.Errorf("UTF16 spans\n got %s\nwant %s", a, b)
			}
			if !reflect.DeepEqual(got.UpstreamSources, fixture.UpstreamSources) {
				a, _ := json.Marshal(got.UpstreamSources)
				b, _ := json.Marshal(fixture.UpstreamSources)
				t.Errorf("raw spans\n got %s\nwant %s", a, b)
			}
			if next := got.IDs.Next(); next != fixture.NextID {
				t.Errorf("next ID got%s want%s", next, fixture.NextID)
			}
			if modules, sources, err := ParseModulesWithSource(fixture.Name+".qnt", fixture.Source); err == nil || modules != nil || sources != nil {
				t.Fatal("ordinary parser lost fail-fast behavior", err)
			}
		})
	}
}
