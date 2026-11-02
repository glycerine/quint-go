package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type documentationFixture struct {
	Cases []struct {
		Source  string
		Modules []struct {
			Name     string
			Entries  []DocumentationEntry
			Markdown string
		}
	}
	Builtin []DocumentationEntry
}

func readDocumentationFixture(t *testing.T) documentationFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/documentation.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture documentationFixture
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestUpstreamDocumentation(t *testing.T) {
	t.Parallel()
	fixture := readDocumentationFixture(t)
	for _, example := range fixture.Cases {
		modules, err := ParseModules(example.Source)
		if err != nil {
			t.Fatal(err)
		}
		if len(modules) != len(example.Modules) {
			t.Fatal("module count", len(modules), len(example.Modules))
		}
		for i, module := range modules {
			want := example.Modules[i]
			entries := ProduceDocs(module)
			got := []DocumentationEntry{}
			markdown := []string{}
			for _, entry := range entries.All() {
				got = append(got, entry)
				markdown = append(markdown, DocumentationMarkdown(entry))
			}
			if !reflect.DeepEqual(got, want.Entries) {
				t.Fatalf("%s entries\ngot  %#v\nwant %#v", module.Name, got, want.Entries)
			}
			if rendered := "# Documentation for " + module.Name + "\n\n" + strings.Join(markdown, "\n\n") + "\n"; rendered != want.Markdown {
				t.Fatalf("%s markdown\ngot  %q\nwant %q", module.Name, rendered, want.Markdown)
			}
			byID := ProduceDocsByID(module)
			for _, declaration := range module.Declarations {
				entry, exists := byID.Get2(declaration.ID)
				if isDocumentationDefinition(declaration) {
					if !exists || entry.Name != declaration.Name {
						t.Fatal("ID documentation", declaration.ID, declaration.Name)
					}
				} else if exists {
					t.Fatal("import/export documentation", declaration.Kind)
				}
			}
		}
	}
}

func TestDocumentationCopiesAndDuplicateIdentity(t *testing.T) {
	modules, err := ParseModules("module M { /// first\nval x=1\nval y=2\n/// last\nval x=3 }")
	if err != nil {
		t.Fatal(err)
	}
	module := modules[0]
	entries := ProduceDocs(module)
	names := []string{}
	for name := range entries.All() {
		names = append(names, name)
	}
	if !reflect.DeepEqual(names, []string{"x", "y"}) || *entries.Get("x").Documentation != "last" {
		t.Fatal("duplicate replacement moved name or retained first doc")
	}
	*entries.Get("x").Documentation = "changed"
	byID := ProduceDocsByID(module)
	if *byID.Get(module.Declarations[2].ID).Documentation != "last" || *module.Declarations[0].Doc != "first" {
		t.Fatal("name and ID documentation share input ownership")
	}
	*byID.Get(module.Declarations[0].ID).Documentation = "changed again"
	if *module.Declarations[0].Doc != "first" {
		t.Fatal("ID documentation mutated input")
	}
	if got := DocumentationMarkdown(DocumentationEntry{Name: "x", Signature: "val x"}); got != "## x\n\nSignature: `val x`\n\n" {
		t.Fatal("missing doc paragraph", got)
	}
	empty := ""
	if got := DocumentationMarkdown(DocumentationEntry{Name: "x", Signature: "val x", Documentation: &empty}); got != "## x\n\nSignature: `val x`\n\n" {
		t.Fatal("empty doc paragraph", got)
	}
	if names := ProduceDocs(Module{}); names.Len() != 0 {
		t.Fatal("empty module documentation")
	}
}

func TestUpstreamBuiltinDocumentation(t *testing.T) {
	t.Parallel()
	fixture := readDocumentationFixture(t)
	entries, err := BuiltinDocs()
	if err != nil {
		t.Fatal(err)
	}
	got := []DocumentationEntry{}
	for _, entry := range entries.All() {
		got = append(got, entry)
	}
	if !reflect.DeepEqual(got, fixture.Builtin) {
		t.Fatal("embedded builtin docs differ from upstream fixture")
	}
	first := entries.Get("Nat")
	*first.Documentation = "modified"
	entries.Clear()
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			copy, err := BuiltinDocs()
			if err != nil || copy.Len() != len(fixture.Builtin) || *copy.Get("Nat").Documentation == "modified" {
				t.Errorf("builtin cache ownership: %v", err)
			}
		})
	}
	group.Wait()
}
