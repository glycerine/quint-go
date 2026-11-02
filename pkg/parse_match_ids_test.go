package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamMatchParserIDs(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, PriorSource string
		Modules                   []Module
		Diagnostics               []Diagnostic
		Sources                   SourceMap
		UpstreamSources           UpstreamSourceMap
		NextID                    ID
	}
	data, err := os.ReadFile("testdata/parser/parser_match_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 25 {
		t.Fatalf("match fixture inventory: got %d, want 25", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ids := NewIDGenerator()
			if fixture.PriorSource != "" {
				prior, err := parseModulesRecoveringWithIDs(fixture.Name+"-prior.qnt", fixture.PriorSource, ids)
				if err != nil || len(prior.Diagnostics) != 0 || prior.IDs != ids {
					t.Fatal("prior parse did not retain the caller's ID sequence", err, prior.Diagnostics)
				}
			}
			parsed, err := parseModulesRecoveringWithIDs(fixture.Name+".qnt", fixture.Source, ids)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(parsed.Modules, fixture.Modules) {
				actual, _ := json.Marshal(parsed.Modules)
				expected, _ := json.Marshal(fixture.Modules)
				t.Errorf("raw match IR differs:\n got %s\nwant %s", actual, expected)
			}
			if !reflect.DeepEqual(parsed.Sources, fixture.Sources) {
				t.Errorf("match UTF-16 source map differs:\n got %+v\nwant %+v", parsed.Sources, fixture.Sources)
			}
			if !reflect.DeepEqual(parsed.UpstreamSources, fixture.UpstreamSources) {
				t.Errorf("match raw source map differs:\n got %+v\nwant %+v", parsed.UpstreamSources, fixture.UpstreamSources)
			}
			if !reflect.DeepEqual(parsed.Diagnostics, fixture.Diagnostics) {
				t.Errorf("match diagnostics/IDs differ:\n got %+v\nwant %+v", parsed.Diagnostics, fixture.Diagnostics)
			}
			if parsed.IDs != ids || parsed.IDs.Next() != fixture.NextID {
				t.Error("match parser did not continue the caller's ID sequence")
			}
			if fixture.PriorSource == "" && len(fixture.Diagnostics) == 0 {
				public, err := ParseModulesWithLocations(fixture.Name+".qnt", fixture.Source)
				if err != nil || !reflect.DeepEqual(public.Modules, fixture.Modules) ||
					!reflect.DeepEqual(public.Sources, fixture.Sources) || !reflect.DeepEqual(public.UpstreamSources, fixture.UpstreamSources) {
					t.Error("public module parser changed match provenance", err)
				}
			}
		})
	}
}

func TestMatchParserProvenanceOwnership(t *testing.T) {
	t.Parallel()
	const source = "module M { val n=match v { A(x)=>match x { B(y)=>y | _=>0 } | _=>1 } }"
	first, err := ParseModulesWithLocations("match.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseModulesWithLocations("match.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	expr := first.Modules[0].Declarations[0].Expr
	expr.Args[1].Str = "changed"
	expr.Args[2].Params[0].Name = "changed"
	expr.Args[2].Body.Args[2].Body.Name = "changed"
	first.Sources[expr.ID].End.Index = 0
	first.UpstreamSources[expr.ID].End.Index = 0
	delete(first.Sources, expr.Args[2].ID)
	delete(first.UpstreamSources, expr.Args[2].Params[0].ID)
	after, err := json.Marshal(second)
	if err != nil || string(before) != string(after) {
		t.Fatal("match parses share expression nodes or provenance maps", err)
	}
	third, err := ParseModulesWithLocations("match.qnt", source)
	if err != nil || !reflect.DeepEqual(second.Modules, third.Modules) ||
		!reflect.DeepEqual(second.Sources, third.Sources) || !reflect.DeepEqual(second.UpstreamSources, third.UpstreamSources) {
		t.Fatal("match parser retained another parse's mutation", err)
	}
}

func TestMatchParserMalformedInputFailsFast(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"match v { A(x)=> }", "match v { A(x)=>1+ }"} {
		if _, err := ParseExpression(expr); err == nil {
			t.Fatal("ordinary expression parser accepted malformed match", expr)
		}
		modules, sources, err := ParseModulesWithSource("malformed-match.qnt", "module Good { val x=1 } module Bad { val n="+expr+" }")
		if err == nil || modules != nil || sources != nil {
			t.Fatal("ordinary module parser retained malformed match IR", modules, sources, err)
		}
	}
}
