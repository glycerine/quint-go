package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

type parserEntryFixture struct {
	Entry, Name, Source, Thrown string
	Result                      json.RawMessage
	UpstreamSources             UpstreamSourceMap
	NextID                      ID
	Recognition                 struct {
		Issues []struct {
			Message string
			Start   UpstreamPosition
		}
		Trailing *struct {
			Text  string
			Start UpstreamPosition
		}
	}
}

func parserEntryDecode(t *testing.T, wire []byte) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(bytes.NewReader(wire))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func parserEntryEqual(t *testing.T, got any, want json.RawMessage) {
	t.Helper()
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parserEntryDecode(t, wire), parserEntryDecode(t, want)) {
		t.Errorf("raw entrypoint output\n got%s\nwant%s", wire, want)
	}
}

func TestUpstreamPublicParserEntries(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/parser/parser_entry.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []parserEntryFixture
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 63 {
		t.Fatal("public parser entry inventory", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Entry+"/"+fixture.Name, func(t *testing.T) {
			if fixture.Entry == "type" {
				testUpstreamTypeEntry(t, fixture)
				return
			}
			filename := fixture.Name + ".qnt"
			file, err := NewSourceFile(filename, fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			located := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
			result, err := parseREPLWithIDs(fixture.Source, located)
			if fixture.Entry == "syntax" {
				parserEntrySyntax(t, fixture, err)
				if result.Kind != "" || len(located.sources) != 0 || len(located.upstreamSources) != 0 || located.generator.Next().String() != "1" {
					t.Fatal("fail-fast syntax conversion must return no partial IR or allocated IDs")
				}
				return
			}
			if fixture.Entry == "semantic" {
				var expected struct{ Errors []Diagnostic }
				if e := json.Unmarshal(fixture.Result, &expected); e != nil {
					t.Fatal(e)
				}
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || len(expected.Errors) == 0 {
					t.Fatal("expected semantic diagnostic", err)
				}
				want, _ := json.Marshal(expected.Errors[0])
				parserEntryEqual(t, diagnostic, want)
				if diagnostic.Reference == nil || !reflect.DeepEqual(located.upstreamSources[*diagnostic.Reference], fixture.UpstreamSources[*diagnostic.Reference]) {
					t.Fatal("semantic reference source changed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wire := map[string]any{"kind": result.Kind}
			if result.Kind == "expr" {
				wire["expr"] = result.Expr
			}
			if result.Kind == "declaration" {
				wire["decls"] = result.Declarations
			}
			parserEntryEqual(t, wire, fixture.Result)
			if !reflect.DeepEqual(located.upstreamSources, fixture.UpstreamSources) {
				t.Error("raw entrypoint sources differ")
			}
			if next := located.generator.Next(); next != fixture.NextID {
				t.Errorf("next ID got%s want%s", next, fixture.NextID)
			}
			public, sources, err := ParseREPLWithSource(filename, fixture.Source, nil)
			if err != nil || !reflect.DeepEqual(public, result) || !reflect.DeepEqual(sources, located.sources) {
				t.Fatal("public source REPL differs", err)
			}
			plain, err := ParseExpressionOrDeclaration(fixture.Source, nil)
			if err != nil || !reflect.DeepEqual(plain, result) {
				t.Fatal("plain REPL differs", err)
			}
			switch fixture.Entry {
			case "expression":
				expr, m, err := ParseExpressionWithSource(filename, fixture.Source)
				if err != nil || !reflect.DeepEqual(&expr, result.Expr) || !reflect.DeepEqual(m, sources) {
					t.Fatal("standalone source expression differs", err)
				}
				expr, err = ParseExpression(fixture.Source)
				if err != nil || !reflect.DeepEqual(&expr, result.Expr) {
					t.Fatal("standalone expression differs", err)
				}
			case "declaration":
				decls, m, err := ParseDeclarationWithSource(filename, fixture.Source)
				if err != nil || !reflect.DeepEqual(decls, result.Declarations) || !reflect.DeepEqual(m, sources) {
					t.Fatal("standalone source declaration differs", err)
				}
				decls, err = ParseDeclaration(fixture.Source)
				if err != nil || !reflect.DeepEqual(decls, result.Declarations) {
					t.Fatal("standalone declaration differs", err)
				}
			}
		})
	}
}

func parserEntrySyntax(t *testing.T, fixture parserEntryFixture, err error) {
	t.Helper()
	var syntax *SyntaxErrors
	if !errors.As(err, &syntax) {
		t.Fatal("expected syntax error", err)
	}
	expected := fixture.Recognition.Issues
	count := len(expected)
	if fixture.Recognition.Trailing != nil {
		count++
	}
	if len(syntax.Issues) != count {
		t.Fatalf("syntax issue count got%v want%d", syntax.Issues, count)
	}
	for i, issue := range expected {
		got := syntax.Issues[i]
		if got.Message != issue.Message {
			t.Errorf("issue%d message got%q want%q", i, got.Message, issue.Message)
		}
		if got.Position != parserEntryPosition(fixture.Source, issue.Start) {
			t.Errorf("issue%d position got%v want%v", i, got.Position, issue.Start)
		}
	}
	if trailing := fixture.Recognition.Trailing; trailing != nil {
		issue := syntax.Issues[count-1]
		if issue.Message != "unexpected trailing input" || issue.Position != parserEntryPosition(fixture.Source, trailing.Start) {
			t.Errorf("Go complete-input boundary changed: %+v", issue)
		}
	}
}

func parserEntryPosition(source string, raw UpstreamPosition) Position {
	position, runeColumn := Position{}, 0
	for _, r := range source {
		if position.Line == raw.Line && runeColumn == raw.Col {
			break
		}
		width := 1
		if r > 0xffff {
			width = 2
		}
		position.Index += width
		if r == '\n' {
			position.Line++
			position.Col = 0
			runeColumn = 0
		} else {
			position.Col += width
			runeColumn++
		}
	}
	return position
}

func testUpstreamTypeEntry(t *testing.T, fixture parserEntryFixture) {
	t.Helper()
	var expected struct {
		Kind  string
		Value json.RawMessage
	}
	if err := json.Unmarshal(fixture.Result, &expected); err != nil {
		t.Fatal(err)
	}
	typ, _, err := ParseTypeWithSource(fixture.Name+".qnt", fixture.Source)
	if fixture.Recognition.Trailing != nil || len(fixture.Recognition.Issues) > 0 {
		parserEntrySyntax(t, fixture, err)
		return
	}
	if expected.Kind == "right" {
		if err != nil {
			t.Fatal(err)
		}
		parserEntryEqual(t, typ, expected.Value)
		plain, err := ParseType(fixture.Source)
		if err != nil {
			t.Fatal(err)
		}
		if err = WalkType(&typ, func(typ *Type) (bool, error) { typ.ID = nil; return true, nil }); err != nil {
			t.Fatal(err)
		}
		wire, _ := json.Marshal(typ)
		parserEntryEqual(t, plain, wire)
		return
	}
	var diagnostics []Diagnostic
	if e := json.Unmarshal(expected.Value, &diagnostics); e != nil || len(diagnostics) == 0 {
		t.Fatal("unclassified reference type failure", string(expected.Value))
	}
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatal("expected type diagnostic", err)
	}
	want, _ := json.Marshal(diagnostics[0])
	parserEntryEqual(t, diagnostic, want)
	if _, err = ParseType(fixture.Source); err == nil || !strings.Contains(err.Error(), diagnostic.Message) {
		t.Fatal("plain type semantic boundary changed", err)
	}
}
