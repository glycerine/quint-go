package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamInvokedIdentifierCompletions(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Main  string
		Files       []struct{ Path, Text string }
		Position    Position
		TriggerKind int
		Result      []semanticCompletionItem
	}
	data, err := os.ReadFile("testdata/identifier_completion.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 6 {
		t.Fatalf("expected six actual upstream completion callbacks, got %d", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.TriggerKind != 1 {
				t.Fatal("fixture did not invoke completion explicitly")
			}
			files := insdict.NewDict[string, string]()
			for _, file := range fixture.Files {
				files.Set(filepath.Join("/identifier-completion", file.Path), file.Text)
			}
			snapshot := semanticTestSnapshot(t, filepath.Join("/identifier-completion", fixture.Main), files)
			before := snapshot.Text
			got, handled := semanticTypedIdentifierCompletions(snapshot, buildSemanticIndex(snapshot), fixture.Position)
			if len(got.Items) != len(fixture.Result) {
				t.Fatalf("Go has %d items; upstream has %d", len(got.Items), len(fixture.Result))
			}
			for i, expected := range fixture.Result {
				// The locked editor dependency is Quint 0.32.0. The repository
				// 0.33.1 builtin.qnt corrects this example's arrow; preserve
				// current embedded documentation and keep the raw oracle intact.
				if expected.Label == "select" && expected.Documentation != nil {
					corrected := strings.Replace(expected.Documentation.Value, "select(x -> x % 2", "select(x => x % 2", 1)
					if corrected == expected.Documentation.Value {
						t.Fatal("upstream documentation correction no longer applies")
					}
					expected.Documentation = &semanticMarkup{Kind: expected.Documentation.Kind, Value: corrected}
				}
				if !reflect.DeepEqual(got.Items[i], expected) {
					actual, _ := json.Marshal(got.Items[i])
					want, _ := json.Marshal(expected)
					t.Fatalf("item %d: Go %s\nupstream %s", i, actual, want)
				}
			}
			// Upstream's empty top-level value result leaves Go's preexisting
			// name-prefix extension available; typed locals/variables take over.
			if handled != (len(fixture.Result) > 0) {
				t.Fatal("unexpected typed completion eligibility", handled)
			}
			if snapshot.Text != before || snapshot.Version != 1 || snapshot.Sources.Get("/identifier-completion/main.qnt") != before {
				t.Fatal("completion mutated the analyzed document")
			}
		})
	}
}

func typedIdentifierTest(t *testing.T, text string) (*Snapshot, semanticIndex, Position) {
	t.Helper()
	offset := strings.Index(text, "|")
	if offset < 0 {
		t.Fatal("missing cursor")
	}
	text = strings.Replace(text, "|", "", 1)
	files := insdict.NewDict[string, string]()
	files.Set("/identifier-completion/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/identifier-completion/main.qnt", files)
	file, _ := quint.NewSourceFile(snapshot.URI, text)
	position, err := file.PositionAt(offset)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, buildSemanticIndex(snapshot), Position{Line: position.Line, Character: position.Col}
}

func TestTypedIdentifierCompletionBoundariesAndCurrentScopes(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		text    string
		labels  []string
		handled bool
	}{
		{`module M { pure val emoji="😀" pure def f(r:{enabled:bool}):{enabled:bool}={ r| } }`, []string{"fieldNames", "with", "enabled"}, true},
		{`module M { pure val n=1 pure val use={ pure val n=true; n| } }`, []string{"iff", "implies"}, true},
		{`module M { type Config={enabled:bool} pure def f(r:Config):Config={ r| } }`, []string{"fieldNames", "with", "enabled"}, true},
		{`module M { pure def f(r)={ r| } }`, []string{}, true},
		{`module M { var r:int pure val use="r|" }`, []string{}, false},
		{"module M { var r:int // r|\n}", []string{}, false},
		{`module M { var r:int /* r| */ }`, []string{}, false},
		{`module M { var record:int val use=rec|ord }`, []string{}, false},
		{`module A { var r:int } module M { import A.* val use=r| }`, []string{}, false},
	} {
		snapshot, index, position := typedIdentifierTest(t, example.text)
		got, handled := semanticTypedIdentifierCompletions(snapshot, index, position)
		labels := []string{}
		for _, item := range got.Items {
			labels = append(labels, item.Label)
		}
		if handled != example.handled || !reflect.DeepEqual(labels, example.labels) {
			t.Fatal(example.text, handled, labels, "wanted", example.handled, example.labels)
		}
	}
	snapshot, index, position := typedIdentifierTest(t, `module M { pure def f(r:bool):bool={ r| } }`)
	snapshot.Analysis.Types = nil
	if got, handled := semanticTypedIdentifierCompletions(snapshot, index, position); !handled || len(got.Items) != 0 {
		t.Fatal("missing analysis used name suggestions or stale types", handled, got)
	}
	snapshot, index, position = typedIdentifierTest(t, `module M { pure def f(r:bool):bool={ r| } }`)
	snapshot.Analysis.Types = insdict.NewDict[string, quint.TypeScheme]()
	if got, handled := semanticTypedIdentifierCompletions(snapshot, index, position); !handled || len(got.Items) != 0 {
		t.Fatal("missing declaration type used a reference type", handled, got)
	}
	if got, handled := semanticTypedIdentifierCompletions(snapshot, index, Position{Character: 10000}); handled || len(got.Items) != 0 {
		t.Fatal("invalid position was accepted", handled, got)
	}
}
