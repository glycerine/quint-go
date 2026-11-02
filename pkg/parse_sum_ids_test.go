package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type sumInferenceTypeEntry struct {
	ID     string
	Scheme TypeScheme
}

type sumInferenceErrorEntry struct {
	ID    string
	Error ErrorTree
}

func TestUpstreamSumParserIDs(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source    string
		Modules         []Module
		Diagnostics     []Diagnostic
		Sources         SourceMap
		UpstreamSources UpstreamSourceMap
		NextID          ID
		Inference       *struct {
			Resolved          []Declaration
			Types             []sumInferenceTypeEntry
			Errors            []sumInferenceErrorEntry
			ApplicationErrors []sumInferenceErrorEntry
		}
	}
	data, err := os.ReadFile("testdata/parser/parser_sum_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 28 {
		t.Fatalf("sum parser fixture inventory: got %d, want 28", len(fixtures))
	}
	inferred := 0
	for _, fixture := range fixtures {
		if fixture.Inference != nil {
			inferred++
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			parsed, err := ParseModulesRecovering(fixture.Name+".qnt", fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(parsed.Modules, fixture.Modules) {
				got, _ := json.Marshal(parsed.Modules)
				want, _ := json.Marshal(fixture.Modules)
				t.Errorf("raw sum IR differs:\n got %s\nwant %s", got, want)
			}
			if !reflect.DeepEqual(parsed.Sources, fixture.Sources) {
				t.Errorf("sum UTF-16 source map differs:\n got %+v\nwant %+v", parsed.Sources, fixture.Sources)
			}
			if !reflect.DeepEqual(parsed.UpstreamSources, fixture.UpstreamSources) {
				t.Errorf("sum raw source map differs:\n got %+v\nwant %+v", parsed.UpstreamSources, fixture.UpstreamSources)
			}
			if !reflect.DeepEqual(parsed.Diagnostics, fixture.Diagnostics) {
				t.Errorf("sum diagnostics/IDs differ:\n got %+v\nwant %+v", parsed.Diagnostics, fixture.Diagnostics)
			}
			if next := parsed.IDs.Next(); next != fixture.NextID {
				t.Errorf("next ID: got %s, want %s", next, fixture.NextID)
			}
			if len(fixture.Diagnostics) > 0 {
				if modules, spans, err := ParseModulesWithSource(fixture.Name+".qnt", fixture.Source); err == nil || modules != nil || spans != nil {
					t.Error("ordinary sum parsing lost fail-fast behavior", err)
				}
			}
			if fixture.Inference == nil {
				return
			}
			resolver := &fixtureSourceResolver{sources: map[string]string{fixture.Name + ".qnt": fixture.Source}}
			project, err := ParseProjectRecovering(resolver, resolver.LookupPath(".", fixture.Name+".qnt"))
			if err != nil || len(project.Diagnostics) != 0 {
				t.Fatal(err, project.Diagnostics)
			}
			if !reflect.DeepEqual(project.Sources, fixture.Sources) || !reflect.DeepEqual(project.UpstreamSources, fixture.UpstreamSources) {
				t.Error("project frontend changed sum provenance")
			}
			analysis, err := AnalyzeModuleTypes(project.Modules, project.Table)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(analysis.Modules[0].Declarations, fixture.Inference.Resolved) {
				got, _ := json.Marshal(analysis.Modules[0].Declarations)
				want, _ := json.Marshal(fixture.Inference.Resolved)
				t.Errorf("resolved sum declarations differ:\n got %s\nwant %s", got, want)
			}
			types := []sumInferenceTypeEntry{}
			for id, scheme := range analysis.Types.All() {
				types = append(types, sumInferenceTypeEntry{id, scheme})
			}
			if !reflect.DeepEqual(types, fixture.Inference.Types) {
				got, _ := json.Marshal(types)
				want, _ := json.Marshal(fixture.Inference.Types)
				t.Errorf("raw inferred sum types/IDs differ:\n got %s\nwant %s", got, want)
			}
			errors, applications := []sumInferenceErrorEntry{}, []sumInferenceErrorEntry{}
			for id, failure := range analysis.TypeErrors.All() {
				errors = append(errors, sumInferenceErrorEntry{id, failure})
			}
			for id, failure := range analysis.ApplicationErrors.All() {
				applications = append(applications, sumInferenceErrorEntry{id, failure})
			}
			if !reflect.DeepEqual(errors, fixture.Inference.Errors) || !reflect.DeepEqual(applications, fixture.Inference.ApplicationErrors) {
				t.Errorf("sum inference errors differ: got %+v/%+v, want %+v/%+v", errors, applications, fixture.Inference.Errors, fixture.Inference.ApplicationErrors)
			}
		})
	}
	if inferred != 6 {
		t.Fatalf("sum inference inventory: got %d, want 6", inferred)
	}
}

func TestSumParserProvenanceOwnership(t *testing.T) {
	t.Parallel()
	const source = "module M { type T[a] = A(List[a]) | B }"
	first, err := ParseModulesWithLocations("sum.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseModulesWithLocations("sum.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	d := first.Modules[0].Declarations
	*d[0].Type.Fields.Fields[0].Type.ID = ID{}
	d[0].Params[0] = "changed"
	d[1].TypeAnnotation.Res.Ctor.Name = "changed"
	d[1].Expr.Params[0].Name = "changed"
	d[2].Expr.Args[0].Str = "changed"
	first.Sources[d[0].ID].End.Index = 0
	first.UpstreamSources[d[0].ID].End.Index = 0
	delete(first.Sources, d[1].ID)
	delete(first.UpstreamSources, d[2].ID)
	after, err := json.Marshal(second)
	if err != nil || string(before) != string(after) {
		t.Fatal("sum expansion shares IR, provenance or source maps across parses", err)
	}
	third, err := ParseModulesWithLocations("sum.qnt", source)
	if err != nil || !reflect.DeepEqual(third.Modules, second.Modules) || !reflect.DeepEqual(third.Sources, second.Sources) || !reflect.DeepEqual(third.UpstreamSources, second.UpstreamSources) {
		t.Fatal("sum expansion retained another parse's mutation", err)
	}
}
