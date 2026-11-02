package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/antlr4-go/antlr/v4"
	"github.com/glycerine/insdict"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// Preserve the previous parser construction as an independent reference for
// exact comparisons, including malformed input's LL diagnostics and recovery.
func moduleTreeLL(text string, listener *effectSyntaxErrors) p.IModulesContext {
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	return parser.Modules()
}

type modulePredictionSnapshot struct {
	Modules                     []Module
	Sources                     SourceMap
	UpstreamSources             UpstreamSourceMap
	Diagnostics                 []Diagnostic
	NextID                      ID
	ErrorType, Error, ErrorJSON string
}

func modulePredictionParse(t testing.TB, source string, recoverSyntax, fast bool, generator *IDGenerator) modulePredictionSnapshot {
	t.Helper()
	file, err := NewSourceFile("parity.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	located := &locatedParserIDs{generator: generator, file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
	var ids parserIDAllocator = located
	listener := newSyntaxErrors(source)
	var recovery *recoveringParserIDs
	if recoverSyntax {
		recovery = &recoveringParserIDs{parserIDAllocator: located}
		ids = recovery
		listener.onIssue = recovery.syntaxIssue
	}
	var tree p.IModulesContext
	if fast {
		tree = parseModuleTree(source, listener)
	} else {
		tree = moduleTreeLL(source, listener)
	}
	modules, err := modulesFromContext(tree, listener, ids)
	if err == nil {
		err = located.err
	}
	result := modulePredictionSnapshot{Modules: modules, Sources: located.sources, UpstreamSources: located.upstreamSources, NextID: located.generator.Next()}
	if recovery != nil {
		result.Diagnostics = recovery.diagnostics
	}
	if err != nil {
		err = withSyntaxSource(err, "parity.qnt")
		result.ErrorType, result.Error = fmt.Sprintf("%T", err), err.Error()
		data, jsonErr := json.Marshal(err)
		if jsonErr != nil {
			t.Fatal(jsonErr)
		}
		result.ErrorJSON = string(data)
	}
	return result
}

func checkModulePredictionParity(t *testing.T, source string) {
	t.Helper()
	for _, recovery := range []bool{false, true} {
		got := modulePredictionParse(t, source, recovery, true, NewIDGenerator())
		want := modulePredictionParse(t, source, recovery, false, NewIDGenerator())
		if !reflect.DeepEqual(got, want) {
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			t.Fatalf("recovery=%v\nsource=%q\nSLL %s\n LL %s", recovery, source, a, b)
		}
	}
}

func TestModulePredictionParity(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name, source string
		fast         bool
	}{
		{"literal", "module M { val x=1 }", true},
		{"annotation", "module M { type Box[a] = List[a] const c:Box[int] val x=c }", true},
		{"deprecated-signature", "module M { def f(x)=x val y=f(1) }", true},
		{"tuple-lambda", "module M { val f=((x,y))=>x+y }", true},
		{"sum-match", "module M { type T=A(int)|B val x=match A(1) { A(n)=>n | B=>0 } }", true},
		{"unicode", "module M { val x=\"😀\" }", true},
		{"conversion-diagnostic", "module M { var x:Map[int,str] }", true},
		{"empty", "", false},
		{"missing-token", "module M { val x=1", false},
		{"missing-expression", "module M { val x= }", false},
		{"trailing-token", "module M {} trailing", false},
		{"trailing-lexical-error", "module M {} @", false},
		{"unterminated-string", "module M {} \"unterminated", false},
		{"equality-action", "module M { val x=1 = 2 val y=3 }", false},
		{"qualified-label-action", "module M { type T=ns::A(int)|B val next=1 }", false},
		{"type-application-action", "module M { var x:Foo(int) val z=1 }", false},
		{"listener-abort", "module Good { val x=1 } module Bad { var x:val y=2 }", false},
		{"operand-recovery", "module M { val x=1+val y=2 }", false},
		{"unsupported-recovery", "module Good { val x=1 } module Bad { val r={a:1,b:val y=2} }", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			if _, fast := parseModuleTreeSLL(fixture.source); fast != fixture.fast {
				t.Fatalf("speculative success=%v want %v", fast, fixture.fast)
			}
			checkModulePredictionParity(t, fixture.source)
		})
	}
}

func TestModulePredictionContinuingIDs(t *testing.T) {
	t.Parallel()
	fast, baseline := NewIDGenerator(), NewIDGenerator()
	for _, source := range []string{
		"module M { val x=1+val y=2 }",
		"module M { val x=\"😀\" }",
		"module Good { val x=1 } module Bad { var x:val y=2 }",
		"module M { var x:Foo(int) val z=1 }",
		"module M { type T=A(int)|B val x=A(1) }",
	} {
		got := modulePredictionParse(t, source, true, true, fast)
		want := modulePredictionParse(t, source, true, false, baseline)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("continuing IDs/maps differ after %q\n got %+v\nwant %+v", source, got, want)
		}
	}
}

func TestModulePredictionFixtureCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for exact LL/SLL fixture comparisons")
	}
	paths, err := filepath.Glob("testdata/parser/*.json")
	if err != nil {
		t.Fatal(err)
	}
	sources := insdict.NewDict[string, bool]()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err = decoder.Decode(&value); err != nil {
			t.Fatal(path, err)
		}
		var visit func(any)
		visit = func(value any) {
			switch v := value.(type) {
			case map[string]any:
				keys := make([]string, 0, len(v))
				for key := range v {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					child := v[key]
					if strings.EqualFold(key, "source") {
						if source, ok := child.(string); ok && strings.Contains(source, "module ") {
							sources.Set(source, true)
						}
					}
					visit(child)
				}
			case []any:
				for _, child := range v {
					visit(child)
				}
			}
		}
		visit(value)
	}
	t.Logf("Comparing %d distinct module sources with ordinary and recovering parsers", sources.Len())
	for source := range sources.All() {
		checkModulePredictionParity(t, source)
	}
}

func TestModulePredictionRepositoryCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for exact LL/SLL repository comparisons")
	}
	count := 0
	for _, root := range []string{"../quint", "../examples", "../evaluator", "../docs", "../skills"} {
		root, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			if entry.IsDir() || filepath.Ext(path) != ".qnt" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			checkModulePredictionParity(t, string(data))
			count++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Compared %d repository Quint source files", count)
}
