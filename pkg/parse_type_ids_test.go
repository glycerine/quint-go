package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamTypeParserIDs(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Kind, Name, Source string
		Type               *Type
		Modules            []Module
		Diagnostics        []Diagnostic
		Sources            SourceMap
		UpstreamSources    UpstreamSourceMap
		NextID             ID
	}
	data, err := os.ReadFile("testdata/parser/parser_type_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 35 {
		t.Fatalf("type parser fixture inventory: got %d, want 35", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Kind+"/"+fixture.Name, func(t *testing.T) {
			filename := fixture.Name + ".qnt"
			var sources SourceMap
			var upstream UpstreamSourceMap
			var generator *IDGenerator
			var diagnostics []Diagnostic
			if fixture.Kind == "type" {
				file, err := NewSourceFile(filename, fixture.Source)
				if err != nil {
					t.Fatal(err)
				}
				located := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
				ids := &recoveringParserIDs{parserIDAllocator: located}
				got, err := parseTypeWithIDs(fixture.Source, ids)
				if err != nil || located.err != nil {
					t.Fatal(err, located.err)
				}
				if !reflect.DeepEqual(got, *fixture.Type) {
					actual, _ := json.Marshal(got)
					expected, _ := json.Marshal(fixture.Type)
					t.Errorf("type IDs/shape differ:\n got %s\nwant %s", actual, expected)
				}
				sources, upstream, generator, diagnostics = located.sources, located.upstreamSources, located.generator, ids.diagnostics
				if len(fixture.Diagnostics) == 0 {
					public, publicSources, err := ParseTypeWithSource(filename, fixture.Source)
					if err != nil || !reflect.DeepEqual(public, *fixture.Type) || !reflect.DeepEqual(publicSources, fixture.Sources) {
						t.Error("public type parser differs", err)
					}
					plain, err := ParseType(fixture.Source)
					if err != nil {
						t.Fatal(err)
					}
					expected, err := ApplySubstitution(nil, *fixture.Type)
					if err != nil {
						t.Fatal(err)
					}
					if err := WalkType(&expected, func(typ *Type) (bool, error) { typ.ID = nil; return true, nil }); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(plain, expected) {
						t.Error("ParseType must preserve the reference shape and omit every type ID")
					}
				}
			} else if fixture.Kind == "module" {
				got, err := ParseModulesRecovering(filename, fixture.Source)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.Modules, fixture.Modules) {
					actual, _ := json.Marshal(got.Modules)
					expected, _ := json.Marshal(fixture.Modules)
					t.Errorf("annotated IR IDs/shape differ:\n got %s\nwant %s", actual, expected)
				}
				sources, upstream, generator, diagnostics = got.Sources, got.UpstreamSources, got.IDs, got.Diagnostics
				if len(fixture.Diagnostics) > 0 {
					if modules, spans, err := ParseModulesWithSource(filename, fixture.Source); err == nil || modules != nil || spans != nil {
						t.Error("ordinary module source parser lost fail-fast behavior", err)
					}
				}
			} else {
				t.Fatal("unknown fixture kind", fixture.Kind)
			}
			if !reflect.DeepEqual(sources, fixture.Sources) {
				t.Errorf("public source spans differ:\n got %+v\nwant %+v", sources, fixture.Sources)
			}
			if !reflect.DeepEqual(upstream, fixture.UpstreamSources) {
				t.Errorf("raw source spans differ:\n got %+v\nwant %+v", upstream, fixture.UpstreamSources)
			}
			if len(diagnostics) != len(fixture.Diagnostics) || len(diagnostics) > 0 && !reflect.DeepEqual(diagnostics, fixture.Diagnostics) {
				t.Errorf("diagnostic IDs differ: got %+v, want %+v", diagnostics, fixture.Diagnostics)
			}
			if next := generator.Next(); next != fixture.NextID {
				t.Errorf("next ID: got %s, want %s", next, fixture.NextID)
			}
		})
	}
}

func TestTypeApplicationIDAndSpanOwnership(t *testing.T) {
	t.Parallel()
	const source = "Box[Result[int, str]]"
	first, firstSources, err := ParseTypeWithSource("type.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	second, secondSources, err := ParseTypeWithSource("type.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[*ID]bool{}
	for _, typ := range []*Type{&first, &second} {
		if err := WalkType(typ, func(node *Type) (bool, error) {
			if node.ID == nil || seen[node.ID] {
				t.Fatal("type nodes must own their ID pointers")
			}
			seen[node.ID] = true
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	root, ctor := *first.ID, *first.Ctor.ID
	if !reflect.DeepEqual(firstSources[root], firstSources[ctor]) {
		t.Fatal("application and constructor must cover the full application")
	}
	*first.ID = ID{}
	firstSources[root].End.Index = 0
	if *first.Ctor.ID != ctor || *second.ID != root || secondSources[root].End.Index != len(source) || firstSources[ctor].End.Index != len(source) {
		t.Fatal("mutating an application ID/span changed another node or parse")
	}
	delete(firstSources, ctor)
	if _, ok := secondSources[ctor]; !ok {
		t.Fatal("separate parses share source maps")
	}
}

func TestTypeApplicationLargeIDs(t *testing.T) {
	t.Parallel()
	generator := NewIDGenerator()
	generator.next.SetString("18446744073709551616", 10)
	typ, err := parseTypeWithIDs("Box[int]", generator)
	if err != nil {
		t.Fatal(err)
	}
	if typ.Args[0].ID.String() != "18446744073709551616" || typ.ID.String() != "18446744073709551617" || typ.Ctor.ID.String() != "18446744073709551618" {
		t.Fatal("application IDs were reordered or truncated", typ)
	}
}
