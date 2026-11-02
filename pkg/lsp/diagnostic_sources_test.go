package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type diagnosticSourceGroup struct {
	Source      string       `json:"source"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type diagnosticPublication struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type diagnosticStage struct {
	Grouped   []diagnosticSourceGroup `json:"grouped"`
	Published diagnosticPublication   `json:"published"`
}

func diagnosticStageTest(groups *insdict.Dict[string, []Diagnostic], main, directory string) diagnosticStage {
	stage := diagnosticStage{Grouped: []diagnosticSourceGroup{}, Published: diagnosticPublication{URI: main, Diagnostics: diagnosticsForSource(groups, filepath.Join(directory, main))}}
	for source, diagnostics := range groups.All() {
		if directory != "" {
			source, _ = filepath.Rel(directory, source)
			source = filepath.ToSlash(source)
		}
		stage.Grouped = append(stage.Grouped, diagnosticSourceGroup{Source: source, Diagnostics: diagnostics})
	}
	return stage
}

func diagnosticJSONEqual(t *testing.T, got, want any) {
	t.Helper()
	left, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	right, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("got %s\nwant %s", left, right)
	}
}

func TestDiagnosticOwnershipUpstreamSourcesAndPublications(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/diagnostic_ownership.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name            string
			Files           []struct{ Path, Text string }
			Main            string
			Parsing         diagnosticStage
			Analysis        *diagnosticStage
			AnalysisSkipped bool
			Navigation      *struct {
				Position Position
				Target   struct{ Kind, Name string }
				Modules  []struct {
					Name         string
					Declarations []struct{ Kind, Name string }
				}
			}
		}
		Helpers []struct {
			Name      string
			Main      string
			Errors    []quint.Diagnostic
			SourceMap []struct {
				ID     quint.ID
				Source string
				Start  quint.Position
				End    *quint.Position
			}
			Parsing  diagnosticStage
			Analysis diagnosticStage
		}
	}
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Helpers {
		t.Run(item.Name, func(t *testing.T) {
			sources := quint.SourceMap{}
			for _, source := range item.SourceMap {
				// Saved inputs use the reference parser's inclusive token end.
				if source.End != nil {
					end := *source.End
					end.Col++
					end.Index++
					source.End = &end
				}
				sources[source.ID] = quint.Location{Source: source.Source, Start: source.Start, End: source.End}
			}
			stage := diagnosticStageTest(diagnosticsBySource(sources, item.Errors), item.Main, "")
			diagnosticJSONEqual(t, stage, item.Parsing)
			diagnosticJSONEqual(t, stage, item.Analysis)
		})
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			const directory = "/diagnostic-fixtures"
			files := insdict.NewDict[string, string]()
			for _, file := range item.Files {
				files.Set(filepath.Join(directory, file.Path), file.Text)
			}
			main := filepath.Join(directory, item.Main)
			resolver := &overlayResolver{ctx: context.Background(), files: files, loaded: insdict.NewDict[string, string](), disk: quint.NewFileSourceResolver(nil, nil)}
			frontend, err := quint.ParseProjectRecovering(resolver, quint.SourceLookupPath{NormalizedPath: main, SourceName: main})
			if err != nil {
				t.Fatal(err)
			}
			stage := diagnosticStageTest(diagnosticsBySource(frontend.Sources, frontend.Diagnostics), item.Main, directory)
			diagnosticJSONEqual(t, stage, item.Parsing)
			want := item.Parsing.Published.Diagnostics
			if item.AnalysisSkipped {
				if len(frontend.Diagnostics) == 0 {
					t.Fatal("analysis should stop on parser diagnostics")
				}
			} else {
				analysis, err := quint.AnalyzeModules(frontend.Modules, frontend.Table)
				if err != nil || item.Analysis == nil {
					t.Fatal(err, item.Analysis)
				}
				stage = diagnosticStageTest(diagnosticsBySource(frontend.Sources, analysis.Diagnostics), item.Main, directory)
				diagnosticJSONEqual(t, stage, *item.Analysis)
				want = item.Analysis.Published.Diagnostics
			}
			snapshot, err := analyzeDocument(context.Background(), Document{URI: uriForSource(main), Text: files.Get(main)}, files)
			if err != nil {
				t.Fatal(err)
			}
			diagnosticJSONEqual(t, snapshot.Diagnostics, want)
			if item.Navigation != nil {
				if snapshot.Analysis.Types != nil || snapshot.Documentation != nil || snapshot.Frontend.Table == nil {
					t.Fatal("parser errors must retain parsing without stale analysis")
				}
				result, problem := semanticTestRequest(t, snapshot, "textDocument/definition", item.Navigation.Position, "")
				if problem != nil {
					t.Fatal(problem)
				}
				locations, ok := result.([]Location)
				if !ok || len(locations) != 1 || semanticTestRangeText(t, snapshot, locations[0]) != item.Navigation.Target.Name {
					t.Fatal("recovered navigation differs from upstream", result)
				}
				if len(snapshot.Frontend.Modules) != len(item.Navigation.Modules) {
					t.Fatal("recovered module count differs", snapshot.Frontend.Modules)
				}
				for i, module := range snapshot.Frontend.Modules {
					want := item.Navigation.Modules[i]
					if module.Name != want.Name || len(module.Declarations) != len(want.Declarations) {
						t.Fatal(module, want)
					}
					for j, declaration := range module.Declarations {
						if declaration.Kind != want.Declarations[j].Kind || declaration.Name != want.Declarations[j].Name {
							t.Fatal(declaration, want.Declarations[j])
						}
					}
				}
			}
		})
	}
}

func TestDiagnosticGroupsOwnReferenceSourcesAndFixes(t *testing.T) {
	t.Parallel()
	main, _ := quint.ParseID("1")
	library, _ := quint.ParseID("2")
	unknown, _ := quint.ParseID("999")
	zero := quint.ID{}
	end := quint.Position{Line: 2, Col: 9, Index: 25}
	sources := quint.SourceMap{
		main:    {Source: "main.qnt", Start: quint.Position{Line: 2, Col: 5, Index: 21}, End: &end},
		library: {Source: "lib.qnt", Start: quint.Position{Line: 1, Col: 3, Index: 13}},
		zero:    {Source: "zero.qnt", Start: quint.Position{}},
	}
	fix := &quint.DiagnosticFix{Kind: "replace", Original: "pure val", Replacement: "val"}
	diagnostics := []quint.Diagnostic{
		{Code: "MAIN", Message: "main error", Reference: &main, Trace: []quint.ID{library}, Data: &quint.DiagnosticData{Fix: fix}},
		{Code: "LIB", Message: "library error", Reference: &library, Trace: []quint.ID{main}, Data: &quint.DiagnosticData{Fix: fix}},
		{Code: "MISSING", Reference: &unknown, Trace: []quint.ID{main}},
		{Code: "NOREF", Trace: []quint.ID{main}},
		{Code: "MAIN2", Reference: &main, Trace: []quint.ID{unknown}},
		{Code: "ZERO", Reference: &zero},
	}
	groups := diagnosticsBySource(sources, diagnostics)
	keys := []string{}
	for key := range groups.All() {
		keys = append(keys, key)
	}
	if !reflect.DeepEqual(keys, []string{"main.qnt", "lib.qnt", "zero.qnt"}) {
		t.Fatal(keys)
	}
	got := diagnosticsForSource(groups, "main.qnt")
	if len(got) != 2 || got[0].Code != "MAIN" || got[1].Code != "MAIN2" || got[0].Range != (Range{Start: Position{Line: 2, Character: 5}, End: Position{Line: 2, Character: 9}}) {
		t.Fatal(got)
	}
	imported := diagnosticsForSource(groups, "lib.qnt")
	if len(imported) != 1 || imported[0].Code != "LIB" || imported[0].Range.Start != imported[0].Range.End {
		t.Fatal(imported)
	}
	for _, diagnostic := range append(got, imported...) {
		if len(diagnostic.RelatedInformation) != 0 {
			t.Fatal("trace frames became editor diagnostics", diagnostic)
		}
	}
	fix.Replacement = "changed after conversion"
	end.Col = 100
	var data quint.DiagnosticData
	if json.Unmarshal(imported[0].Data, &data) != nil || data.Fix == nil || data.Fix.Replacement != "val" || got[0].Range.End.Character != 9 {
		t.Fatal("diagnostic retained mutable analysis data", imported[0], got[0])
	}
	if diagnosticsForSource(groups, "other.qnt") == nil || len(diagnosticsForSource(groups, "other.qnt")) != 0 {
		t.Fatal("missing source must clear diagnostics")
	}
}

func TestDiagnosticGroupsEmptyAndUTF16Spans(t *testing.T) {
	t.Parallel()
	if groups := diagnosticsBySource(nil, nil); groups.Len() != 0 {
		t.Fatal(groups)
	}
	if got := diagnosticsForSource(nil, "main.qnt"); got == nil || len(got) != 0 {
		t.Fatal(got)
	}
	id, _ := quint.ParseID("1")
	file, err := quint.NewSourceFile("main.qnt", "😀bad\nline")
	if err != nil {
		t.Fatal(err)
	}
	location, err := file.Location(len("😀"), len("😀bad"))
	if err != nil {
		t.Fatal(err)
	}
	groups := diagnosticsBySource(quint.SourceMap{id: location}, []quint.Diagnostic{{Code: "QNT404", Reference: &id, Data: &quint.DiagnosticData{}}})
	got := diagnosticsForSource(groups, "main.qnt")
	if len(got) != 1 || got[0].Range.Start.Character != 2 || got[0].Range.End.Character != 5 || string(got[0].Data) != "{}" {
		t.Fatal(got)
	}
}
