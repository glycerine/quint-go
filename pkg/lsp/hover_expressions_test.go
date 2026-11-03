package lsp

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type hoverSymbolFixture struct {
	Name     string
	Files    []struct{ Path, Text string }
	Main     string
	Position Position
	Modules  []quint.Module
	Sources  []struct {
		ID       quint.ID
		Location quint.Location
	}
	Types []struct {
		ID     quint.ID
		Scheme quint.TypeScheme
	}
	Effects []struct {
		ID     quint.ID
		Scheme quint.EffectScheme
	}
	Result json.RawMessage
}

func hoverSymbolFixtures(t *testing.T) (hovers, symbols []hoverSymbolFixture) {
	t.Helper()
	var fixture struct {
		Hovers  []hoverSymbolFixture
		Symbols []hoverSymbolFixture
	}
	data, err := os.ReadFile("testdata/hover_symbols.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Hovers, fixture.Symbols
}
func hoverSymbolSnapshot(t *testing.T, fixture hoverSymbolFixture) *Snapshot {
	t.Helper()
	snapshot := &Snapshot{URI: "file:///workspace/" + fixture.Main, Version: 1, Frontend: quint.FrontendResult{Modules: fixture.Modules, Sources: quint.SourceMap{}}, Sources: insdict.NewDict[string, string](), Documentation: insdict.NewDict[string, quint.DocumentationEntry]()}
	for _, file := range fixture.Files {
		snapshot.Sources.Set("/workspace/"+file.Path, file.Text)
		if file.Path == fixture.Main {
			snapshot.Text = file.Text
		}
	}
	for _, source := range fixture.Sources {
		snapshot.Frontend.Sources[source.ID] = source.Location
	}
	linked, err := quint.ResolveLinkedModules(fixture.Modules)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Frontend.Table, snapshot.Frontend.Scopes = linked.Table, linked.Modules
	snapshot.Analysis.Types = insdict.NewDict[string, quint.TypeScheme]()
	for _, item := range fixture.Types {
		snapshot.Analysis.Types.Set(item.ID.String(), item.Scheme)
	}
	snapshot.Analysis.Effects = insdict.NewDict[string, quint.EffectScheme]()
	for _, item := range fixture.Effects {
		snapshot.Analysis.Effects.Set(item.ID.String(), item.Scheme)
	}
	for _, module := range fixture.Modules {
		for id, doc := range quint.ProduceDocsByID(module).All() {
			snapshot.Documentation.Set(id.String(), doc)
		}
	}
	return snapshot
}

func TestUpstreamHoverExpressionAndDocumentationFixtures(t *testing.T) {
	t.Parallel()
	fixtures, _ := hoverSymbolFixtures(t)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			snapshot := hoverSymbolSnapshot(t, fixture)
			got := semanticHoverForPosition(snapshot, buildSemanticIndex(snapshot), fixture.Position)
			if string(fixture.Result) == "null" {
				if got != nil {
					t.Fatalf("unexpected hover: %#v", got)
				}
				return
			}
			var want struct{ Contents semanticMarkup }
			if err := json.Unmarshal(fixture.Result, &want); err != nil {
				t.Fatal(err)
			}
			if got == nil || !reflect.DeepEqual(got.(semanticHover).Contents, want.Contents) {
				t.Fatalf("hover got %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestHoverFieldsLambdasAndBuiltinDocsThroughGoAnalysis(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct{ text, needle, want string }{
		{`module Main { pure val r = { count: 1 } pure val value = r.count }`, "count }", "(field) count: int"},
		{`module Main { pure val text = "😀" pure val r = { enabled: true } pure val value = r.enabled }`, "enabled }", "(field) enabled: bool"},
		{`module Main { pure def f = (value) => value + 1 }`, "=>", "(value) => value + 1: (int) => int"},
		{`module Main { pure val value = Set(1).map(value => value + 1) }`, "=>", "value => value + 1: (int) => int"},
		{`module Main { pure val value = size(Set(1)) }`, "size", "`s.size()` is the cardinality"},
		{`module Main { pure val value = 1 + 2 }`, "+", "`a.iadd(b)` is the integer addition"},
	} {
		files := insdict.NewDict[string, string]()
		files.Set("/hover/main.qnt", fixture.text)
		snapshot := semanticTestSnapshot(t, "/hover/main.qnt", files)
		position := semanticTestPosition(t, fixture.text, fixture.needle, 0)
		got, problem := semanticTestRequest(t, snapshot, "textDocument/hover", position, "")
		if problem != nil || got == nil || !strings.Contains(got.(semanticHover).Contents.Value, fixture.want) {
			t.Fatal(fixture.text, got, problem)
		}
		if !semanticContains(got.(semanticHover).Range, position) {
			t.Fatal("hover range does not contain UTF-16 cursor", got, position)
		}
	}
}

func hoverDictionaryJSON[T any](t *testing.T, dictionary *insdict.Dict[string, T]) []byte {
	t.Helper()
	entries := [][2]any{}
	for id, value := range dictionary.All() {
		entries = append(entries, [2]any{id, value})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHoverRenderingDoesNotMutateAnalysis(t *testing.T) {
	t.Parallel()
	files := insdict.NewDict[string, string]()
	text := `module Main { pure def identity = (value) => value }`
	files.Set("/hover/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/hover/main.qnt", files)
	beforeTypes := hoverDictionaryJSON(t, snapshot.Analysis.Types)
	beforeEffects := hoverDictionaryJSON(t, snapshot.Analysis.Effects)
	position := semanticTestPosition(t, text, "=>", 0)
	for range 3 {
		if semanticHoverForPosition(snapshot, buildSemanticIndex(snapshot), position) == nil {
			t.Fatal("missing hover")
		}
	}
	afterTypes := hoverDictionaryJSON(t, snapshot.Analysis.Types)
	afterEffects := hoverDictionaryJSON(t, snapshot.Analysis.Effects)
	if string(beforeTypes) != string(afterTypes) || string(beforeEffects) != string(afterEffects) {
		t.Fatal("hover mutated analysis")
	}
}
