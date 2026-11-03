package lsp

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
)

type overrideSemanticFixture struct {
	Name, URI, Text, NewName string
	Position                 Position
	Definition               []struct {
		TargetURI            string
		TargetSelectionRange Range
	}
	References []Location
	Rename     *struct {
		Changes map[string][]semanticTextEdit
	}
}

func overrideSemanticFixtures(t *testing.T) []overrideSemanticFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/override_semantics.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []overrideSemanticFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 8 {
		t.Fatalf("expected eight actual upstream scoped semantic cases, got %d", len(fixtures))
	}
	return fixtures
}

func TestOverrideScopedSemanticsUpstream(t *testing.T) {
	for _, fixture := range overrideSemanticFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			if strings.Contains(fixture.Name, "utf16") {
				// The reference parser's saved spans count code points. Keep
				// its raw evidence and project it to LSP UTF-16 coordinates;
				// reproducing that old column bug would split editor tokens.
				fixture = overrideUTF16Projection(fixture)
			}
			path, err := sourceForURI(fixture.URI)
			if err != nil {
				t.Fatal(err)
			}
			files := insdict.NewDict[string, string]()
			files.Set(path, fixture.Text)
			snapshot := semanticTestSnapshot(t, path, files)
			response, problem := semanticTestRequest(t, snapshot, "textDocument/definition", fixture.Position, "")
			locations, ok := response.([]Location)
			if problem != nil || !ok || len(locations) != 1 {
				t.Fatalf("definition: %#v; %v", response, problem)
			}
			want := fixture.Definition[0]
			if locations[0].URI != want.TargetURI || locations[0].Range.Start != want.TargetSelectionRange.Start {
				t.Fatalf("definition differs: Go %+v; TypeScript %+v", locations[0], want)
			}
			name := semanticTestRangeText(t, snapshot, locations[0])
			if name == "" || strings.ContainsAny(name, "\n ()=;") {
				t.Fatalf("definition did not select the declaration name: %q", name)
			}
			response, problem = semanticTestRequest(t, snapshot, "textDocument/references", fixture.Position, `,"context":{"includeDeclaration":true}`)
			references, ok := response.([]Location)
			if problem != nil || !ok || len(references) != len(fixture.References) {
				t.Fatalf("references: %#v; %v; want %d", response, problem, len(fixture.References))
			}
			// Upstream reference spans can include call arguments, parameter
			// annotations, or whole type declarations. Preserve Go's existing
			// token precision while checking the same complete set of sites.
			used := make([]bool, len(fixture.References))
			for _, got := range references {
				if semanticTestRangeText(t, snapshot, got) != name {
					t.Fatalf("reference does not select %q: %+v", name, got)
				}
				matched := false
				for i, want := range fixture.References {
					if !used[i] && got.URI == want.URI && overrideRangeContains(want.Range, got.Range) {
						used[i], matched = true, true
						break
					}
				}
				if !matched {
					t.Fatalf("reference is absent upstream: %+v; TypeScript %+v", got, fixture.References)
				}
			}
			newName, _ := json.Marshal(fixture.NewName)
			response, problem = semanticTestRequest(t, snapshot, "textDocument/rename", fixture.Position, `,"newName":`+string(newName))
			if problem != nil {
				t.Fatal(problem)
			}
			var got map[string][]semanticTextEdit
			if err := json.Unmarshal(response.(semanticWorkspaceEdit).Changes, &got); err != nil {
				t.Fatal(err)
			}
			if fixture.Rename != nil {
				sortEdits := func(values map[string][]semanticTextEdit) {
					for _, edits := range values {
						sort.Slice(edits, func(i, j int) bool { return semanticPositionLess(edits[i].Range.Start, edits[j].Range.Start) })
					}
				}
				sortEdits(got)
				sortEdits(fixture.Rename.Changes)
				if len(got) != len(fixture.Rename.Changes) {
					t.Fatalf("rename sources differ: Go %+v; TypeScript %+v", got, fixture.Rename.Changes)
				}
				for uri, edits := range got {
					want := fixture.Rename.Changes[uri]
					if len(edits) != len(want) {
						t.Fatalf("rename sites differ: Go %+v; TypeScript %+v", edits, want)
					}
					for i, edit := range edits {
						if edit.NewText != want[i].NewText || !overrideRangeContains(want[i].Range, edit.Range) || semanticTestRangeText(t, snapshot, Location{URI: uri, Range: edit.Range}) != name {
							t.Fatalf("rename differs: Go %+v; TypeScript %+v", edit, want[i])
						}
					}
				}
			}
			updated := semanticTestApplyRename(t, snapshot, files, fixture.Position, fixture.NewName)
			semanticTestSnapshot(t, path, updated)
			if !strings.Contains(updated.Get(path), fixture.NewName) || files.Get(path) != fixture.Text {
				t.Fatal("rename failed to update the new source or mutated the original")
			}
		})
	}
}

func overrideRangeContains(outer, inner Range) bool {
	return !semanticPositionLess(inner.Start, outer.Start) && !semanticPositionLess(outer.End, inner.End)
}

func overrideUTF16Projection(fixture overrideSemanticFixture) overrideSemanticFixture {
	lines := strings.Split(fixture.Text, "\n")
	position := func(p Position) Position {
		column := 0
		for index, r := range []rune(lines[p.Line]) {
			if index >= p.Character {
				break
			}
			column++
			if r > 0xffff {
				column++
			}
		}
		return Position{Line: p.Line, Character: column}
	}
	rangeProjection := func(value Range) Range {
		return Range{Start: position(value.Start), End: position(value.End)}
	}
	for i := range fixture.Definition {
		fixture.Definition[i].TargetSelectionRange = rangeProjection(fixture.Definition[i].TargetSelectionRange)
	}
	for i := range fixture.References {
		fixture.References[i].Range = rangeProjection(fixture.References[i].Range)
	}
	if fixture.Rename != nil {
		for _, edits := range fixture.Rename.Changes {
			for i := range edits {
				edits[i].Range = rangeProjection(edits[i].Range)
			}
		}
	}
	return fixture
}

func TestOverrideScopedCompletion(t *testing.T) {
	fixture := overrideSemanticFixtures(t)[0]
	path, err := sourceForURI(fixture.URI)
	if err != nil {
		t.Fatal(err)
	}
	files := insdict.NewDict[string, string]()
	files.Set(path, fixture.Text)
	snapshot := semanticTestSnapshot(t, path, files)
	response, problem := semanticTestRequest(t, snapshot, "textDocument/completion", fixture.Position, "")
	if problem != nil {
		t.Fatal(problem)
	}
	for _, item := range response.(semanticCompletionList).Items {
		if item.Label == "value" {
			if item.Detail != "int" {
				t.Fatal("override parameter completion lost its type", item)
			}
			return
		}
	}
	t.Fatal("override parameter is absent from scoped completion", response)
}
