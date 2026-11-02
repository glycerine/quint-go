package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

type sourceRecoveryFixtureStage struct {
	Modules     json.RawMessage
	Diagnostics []struct {
		Code, Message string
		Data          *DiagnosticData
		RawLocation   *UpstreamLocation
		Span          *struct {
			Source     string
			Start, End int
		}
	}
	SourceNames []string
}

func TestUpstreamSourceRecovery(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name            string
		Sources         map[string]string
		Reads           []string
		Loaded, Project sourceRecoveryFixtureStage
	}
	data, err := os.ReadFile("testdata/parser/source_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte) any {
		t.Helper()
		var value any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return withoutParserIDs(value)
	}
	compare := func(t *testing.T, modules []Module, diagnostics []Diagnostic, sources SourceMap, raw UpstreamSourceMap, want sourceRecoveryFixtureStage) {
		t.Helper()
		got, err := json.Marshal(modules)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(decode(got), decode(want.Modules)) {
			t.Fatalf("modules differ\nGo %s\nTS %s", got, want.Modules)
		}
		if len(diagnostics) != len(want.Diagnostics) {
			t.Fatalf("diagnostic count: Go %+v TS %+v", diagnostics, want.Diagnostics)
		}
		for i, want := range want.Diagnostics {
			got := diagnostics[i]
			if got.Code != want.Code || got.Message != want.Message || !reflect.DeepEqual(got.Data, want.Data) {
				t.Fatalf("diagnostic %d: Go %+v TS %+v", i, got, want)
			}
			if want.Span != nil {
				if got.Reference == nil {
					t.Fatal("missing diagnostic reference", got)
				}
				loc, ok := sources[*got.Reference]
				// Upstream reports lexer diagnostics with index zero even at a
				// nonzero column. Public spans retain the true lexer cursor.
				lexerCursor := want.RawLocation.Start.Index == 0 && want.RawLocation.End.Index == 0 && want.RawLocation.Start.Col != 0
				if !ok || loc.Source != want.Span.Source || loc.End == nil || (!lexerCursor && (loc.Start.Index != want.Span.Start || loc.End.Index != want.Span.End)) {
					t.Fatalf("diagnostic %d span: Go %+v TS %+v", i, loc, want.Span)
				}
				if actual, ok := raw[*got.Reference]; !ok || !reflect.DeepEqual(&actual, want.RawLocation) {
					t.Fatal("upstream source span", actual, want.RawLocation)
				}
			}
		}
		unique := map[string]bool{}
		for _, loc := range sources {
			unique[loc.Source] = true
		}
		names := []string{}
		for name := range unique {
			names = append(names, name)
		}
		sort.Strings(names)
		if !reflect.DeepEqual(names, want.SourceNames) {
			t.Fatal("available sources", names, want.SourceNames)
		}
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			resolver := &fixtureSourceResolver{sources: fixture.Sources}
			loaded, err := LoadModulesRecovering(resolver, resolver.LookupPath(".", "main.qnt"))
			if err != nil {
				t.Fatal(err)
			}
			compare(t, loaded.Modules, loaded.Diagnostics, loaded.Sources, loaded.UpstreamSources, fixture.Loaded)
			if !reflect.DeepEqual(resolver.reads, fixture.Reads) {
				t.Fatal("reads", resolver.reads, fixture.Reads)
			}
			resolver = &fixtureSourceResolver{sources: fixture.Sources}
			project, err := ParseProjectRecovering(resolver, resolver.LookupPath(".", "main.qnt"))
			if err != nil {
				t.Fatal(err)
			}
			compare(t, project.Modules, project.Diagnostics, project.Sources, project.UpstreamSources, fixture.Project)
			if !reflect.DeepEqual(resolver.reads, fixture.Reads) {
				t.Fatal("project reads", resolver.reads, fixture.Reads)
			}
			if project.Table == nil || project.Scopes == nil || project.IDs == nil {
				t.Fatal("missing project continuation data")
			}
			if _, exists := project.Sources[project.IDs.Next()]; exists {
				t.Fatal("reused source ID")
			}
		})
	}
}

func TestSourceRecoveryValidationAndOwnership(t *testing.T) {
	t.Parallel()
	if _, err := LoadModulesRecovering(nil, SourceLookupPath{}); err == nil {
		t.Fatal("accepted nil resolver")
	}
	resolver := &fixtureSourceResolver{sources: map[string]string{}}
	main := resolver.LookupPath(".", "main.qnt")
	result, err := LoadModulesRecovering(resolver, main)
	if err == nil || len(result.Modules) != 0 || len(result.Sources) != 0 || len(result.UpstreamSources) != 0 || len(result.Diagnostics) != 0 || result.IDs == nil {
		t.Fatal("main failure", result, err)
	}
	if _, err = ParseProjectRecovering(resolver, main); err == nil {
		t.Fatal("project accepted missing main")
	}
	resolver.sources = map[string]string{"main.qnt": string([]byte{0xff})}
	if _, err = LoadModulesRecovering(resolver, main); err == nil {
		t.Fatal("accepted non-UTF8 main source")
	}
	resolver.sources = map[string]string{"main.qnt": "module Main { import A from \"a\" }", "a.qnt": string([]byte{0xff})}
	if result, err = LoadModulesRecovering(resolver, main); err == nil || len(result.Sources) == 0 {
		t.Fatal("import conversion failure lost main source map", result, err)
	}
	resolver.sources = map[string]string{"main.qnt": "module Main { type lower import A from \"a\" }", "a.qnt": "module A { val x=1 }"}
	first, err := LoadModulesRecovering(resolver, main)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadModulesRecovering(resolver, main)
	if err != nil {
		t.Fatal(err)
	}
	first.Diagnostics[0].Message = "changed"
	first.Modules[0].Name = "changed"
	delete(first.Sources, *first.Diagnostics[0].Reference)
	if second.Diagnostics[0].Message == "changed" || second.Modules[0].Name == "changed" || len(second.Sources) == len(first.Sources) {
		t.Fatal("results share mutable containers")
	}
	seen := map[ID]bool{}
	for _, module := range second.Modules {
		if seen[module.ID] {
			t.Fatal("duplicate module ID")
		}
		seen[module.ID] = true
		for _, declaration := range module.Declarations {
			if seen[declaration.ID] {
				t.Fatal("duplicate declaration ID")
			}
			seen[declaration.ID] = true
		}
	}
}
