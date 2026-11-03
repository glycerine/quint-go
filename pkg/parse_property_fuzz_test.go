package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"unicode/utf8"
)

// Parsing produces the public transport IR. Successful parses must survive its
// strict decoders, and every parsed node must retain its editor source span.
// The seeds exercise expansions and annotations as well as ordinary leaves;
// native fuzzing then mutates syntax rather than only valid JSON trees.
func FuzzParserEntryRoundTrip(f *testing.F) {
	for mode, seeds := range [][]string{
		{"", "module M {}", "module M { type T[a]=A(a)|B val (x,y)=(1,2) }", "module M { import A(c=1,d=2) as X }", "module M { var x }"},
		{"1", "((x,y)) => {val (a,b)=(x,y) a+b}", "{a:1,...r,b:true}", "match x { A(a)=>a | _=>0 }", "1+"},
		{"const x:Result[List[int],{value:bool}]", "type T[a]=A(a)|B", "pure val {x,y}={x:1,y:2}", "def f(x:int):int=x", "var x:"},
		{"{}", "Result[List[int],{value:bool}]", "(int,bool)=>Set[str]", "{|r}", "Set["},
		{"Pure", "(Read['x'],Update['x'])=>Temporal['x']", "Read['x',v,'x']", "Read[,v]", "(Pure)=>"},
		{"", "/// docs\n", "val x:int=1", "{val (x,y)=(1,2) x+y}", "val x="},
	} {
		for _, seed := range seeds {
			f.Add(uint8(mode), seed)
		}
	}
	f.Add(uint8(1), "// 😀\n\"é\"")
	f.Add(uint8(0), "module M { val x=\"\xff\" }")
	f.Fuzz(func(t *testing.T, mode uint8, source string) {
		if len(source) > 512 {
			t.Skip()
		}
		var value any
		var sources SourceMap
		var err error
		switch mode % 6 {
		case 0:
			var modules []Module
			modules, sources, err = ParseModulesWithSource("fuzz.qnt", source)
			if err == nil {
				parserPropertyRoundTrip(t, modules)
				value = modules
			} else if modules != nil || sources != nil {
				t.Fatal("failed strict module parse returned partial output")
			}
		case 1:
			var expr Expr
			expr, sources, err = ParseExpressionWithSource("fuzz.qnt", source)
			if err == nil {
				parserPropertyRoundTrip(t, expr)
				value = expr
			} else if !reflect.DeepEqual(expr, Expr{}) || sources != nil {
				t.Fatal("failed strict expression parse returned partial output")
			}
		case 2:
			var declarations []Declaration
			declarations, sources, err = ParseDeclarationWithSource("fuzz.qnt", source)
			if err == nil {
				parserPropertyRoundTrip(t, declarations)
				value = declarations
			} else if declarations != nil || sources != nil {
				t.Fatal("failed strict declaration parse returned partial output")
			}
		case 3:
			var typ Type
			typ, sources, err = ParseTypeWithSource("fuzz.qnt", source)
			if err == nil {
				if err := typ.Validate(); err != nil {
					t.Fatal("parsed type failed recursive validation", err)
				}
				parserPropertyRoundTrip(t, typ)
				value = typ
			} else if !reflect.DeepEqual(typ, Type{}) || sources != nil {
				t.Fatal("failed strict type parse returned partial output")
			}
		case 4:
			var effect Effect
			effect, err = ParseEffect(source)
			if err == nil {
				parserPropertyRoundTrip(t, effect)
			} else if !reflect.DeepEqual(effect, Effect{}) {
				t.Fatal("failed effect parse returned partial output")
			}
		case 5:
			var result ExpressionOrDeclaration
			result, sources, err = ParseREPLWithSource("fuzz.qnt", source, nil)
			if err == nil {
				switch result.Kind {
				case "declaration":
					parserPropertyRoundTrip(t, result.Declarations)
					value = result.Declarations
				case "expr":
					if result.Expr == nil {
						t.Fatal("expression REPL result has no expression")
					}
					parserPropertyRoundTrip(t, *result.Expr)
					value = result.Expr
				case "none":
					if result.Expr != nil || result.Declarations != nil || len(sources) != 0 {
						t.Fatal("empty REPL result retained IR")
					}
				default:
					t.Fatalf("unknown REPL result kind %q", result.Kind)
				}
			} else if !reflect.DeepEqual(result, ExpressionOrDeclaration{}) {
				t.Fatal("failed REPL parse returned partial IR")
			}
		}
		if !utf8.ValidString(source) {
			if mode%6 != 4 && err == nil {
				t.Fatal("source-aware entry accepted invalid UTF-8")
			}
			return
		}
		file, e := NewSourceFile("fuzz.qnt", source)
		if e != nil {
			t.Fatal(e)
		}
		var syntax *SyntaxErrors
		if errors.As(err, &syntax) {
			for _, issue := range syntax.Issues {
				offset, e := file.ByteOffset(issue.Position.Line, issue.Position.Col)
				if e != nil {
					t.Fatal("syntax diagnostic is not at a source boundary", issue, e)
				}
				position, e := file.PositionAt(offset)
				if e != nil || position != issue.Position {
					t.Fatal("syntax diagnostic index disagrees with line/column", issue, e)
				}
			}
		}
		if value != nil {
			parserPropertySourceCoverage(t, value, sources)
		}
	})
}

func parserPropertyRoundTrip[T any](t *testing.T, value T) {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal("parsed IR cannot be serialized", err)
	}
	var decoded T
	if err = json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal("parsed IR rejected by strict decoder", string(wire), err)
	}
	again, err := json.Marshal(decoded)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatal("parsed IR wire roundtrip changed", string(wire), string(again), err)
	}
}

func parserPropertySourceCoverage(t *testing.T, value any, sources SourceMap) {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(wire))
	decoder.UseNumber()
	if err = decoder.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if number, ok := node["id"].(json.Number); ok {
				id, err := ParseID(number.String())
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := sources[id]; !ok {
					t.Fatal("parsed IR has no public source span", id)
				}
			}
			for _, child := range node {
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(decoded)
}

// The fast strategy may publish no speculative diagnostic or ID allocation.
// Compare complete raw snapshots to the independent LL construction, including
// explicit unsupported-recovery errors and continuing generator state.
func FuzzModulePredictionRecovery(f *testing.F) {
	for _, source := range []string{
		"", "module M {}", "module M { val x=1 }", "module M { val f=((x,y))=>x+y }",
		"module M { val (x,y)=(1,2) }", "module M { type T=A|B(int) }", "module M { var x }",
		"module M { val x=1+ }", "module M { val r={a:1,b:} }", "module Good {} module Bad {val x=}",
		"// 😀\nmodule M { val x=\"é\" }", "module M { import A(c=1,d=2) as X }",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 256 || !utf8.ValidString(source) {
			t.Skip()
		}
		checkModulePredictionParity(t, source)
	})
}
