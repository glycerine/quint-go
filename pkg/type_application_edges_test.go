package quint

import (
	"encoding/json"
	"iter"
	"os"
	"sort"
	"testing"
)

// Compare the complete upstream tables, including every type-node ID, instead
// of comparing only the final type or dropping annotation/builtin provenance.
func TestUpstreamTypeApplicationEdges(t *testing.T) {
	t.Parallel()
	type typeEntry struct {
		ID      string
		Scheme  TypeScheme
		Printed string
	}
	type errorEntry struct {
		ID    string
		Error ErrorTree
	}
	type lookupTypes struct {
		ID               string
		Annotation, Type *Type
	}
	var fixtures []struct {
		Name, Source      string
		Input             []Module
		Resolved          []Declaration
		Table             []lookupTypes
		Types             []typeEntry
		Errors            []errorEntry
		ApplicationErrors []errorEntry
	}
	data, err := os.ReadFile("testdata/parser/type_application_edges.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 36 {
		t.Fatal("missing type application and annotation edge cases")
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			assertJSON := func(label string, got, want any) {
				t.Helper()
				a, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				b, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if string(a) != string(b) {
					t.Errorf("%s Go %s\nTypeScript %s", label, a, b)
				}
			}
			before, err := json.Marshal(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			linked, err := ResolveLinkedModules(fixture.Input)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			analysis, err := AnalyzeModuleTypes(fixture.Input, linked.Table)
			if err != nil {
				t.Fatal(err)
			}
			resolved := []Declaration{}
			for _, module := range analysis.Modules {
				resolved = append(resolved, module.Declarations...)
			}
			assertJSON("resolved declarations", resolved, fixture.Resolved)
			table := []lookupTypes{}
			for id, definition := range linked.Table.All() {
				entry := lookupTypes{ID: id.String()}
				if definition.Declaration != nil {
					entry.Annotation = definition.Declaration.TypeAnnotation
					if definition.Declaration.Kind == "typedef" {
						entry.Type = definition.Declaration.Type
					}
				} else if definition.Parameter != nil {
					entry.Annotation = definition.Parameter.TypeAnnotation
				}
				table = append(table, entry)
			}
			// The saved input is dependency-ordered phase-three IR, whereas
			// upstream populated its lookup table before that declaration sort.
			// Compare complete lookup payloads by ID; schemes and diagnostics
			// below retain their actual inference insertion order.
			sort.Slice(table, func(i, j int) bool { return table[i].ID < table[j].ID })
			sort.Slice(fixture.Table, func(i, j int) bool { return fixture.Table[i].ID < fixture.Table[j].ID })
			assertJSON("resolved lookup types", table, fixture.Table)
			types := []typeEntry{}
			for id, scheme := range analysis.Types.All() {
				printed, err := TypeSchemeString(scheme)
				if err != nil {
					t.Fatal(err)
				}
				types = append(types, typeEntry{ID: id, Scheme: scheme, Printed: printed})
			}
			assertJSON("ordered inferred schemes", types, fixture.Types)
			failures := func(entries iter.Seq2[string, ErrorTree]) []errorEntry {
				out := []errorEntry{}
				for id, failure := range entries {
					out = append(out, errorEntry{ID: id, Error: failure})
				}
				return out
			}
			assertJSON("inference diagnostics", failures(analysis.TypeErrors.All()), fixture.Errors)
			assertJSON("application diagnostics", failures(analysis.ApplicationErrors.All()), fixture.ApplicationErrors)
			after, err := json.Marshal(fixture.Input)
			if err != nil || string(before) != string(after) {
				t.Fatal("analysis mutated source IR", err)
			}
		})
	}
}
