package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
	"testing"
)

func TestParserSourceSpans(t *testing.T) {
	text := "  (\"😀\" +\n \"é\")  "
	file, err := NewSourceFile("unicode.qnt", text)
	if err != nil {
		t.Fatal(err)
	}
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	tree := parser.Expr()
	loc, err := parserLocation(file, tree)
	if err != nil {
		t.Fatal(err)
	}
	if loc.Source != "unicode.qnt" || loc.Start != (Position{Line: 0, Col: 2, Index: 2}) || loc.End == nil || *loc.End != (Position{Line: 1, Col: 5, Index: 15}) {
		t.Fatalf("span %+v end %v", loc, loc.End)
	}
	inner := tree.(*p.ParenContext).Expr().(*p.PlusMinusContext)
	literal, err := parserLocation(file, inner.Expr(0))
	if err != nil {
		t.Fatal(err)
	}
	if literal.Start.Index != 3 || literal.End.Index != 7 {
		t.Fatalf("supplementary literal %+v", literal)
	}
}
func TestParserEOFAndMissingStop(t *testing.T) {
	text := "module M {}\n"
	file, _ := NewSourceFile("m.qnt", text)
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(p.NewQuintLexer(antlr.NewInputStream(text)), antlr.TokenDefaultChannel))
	tree := parser.Modules()
	loc, err := parserLocation(file, tree)
	if err != nil || loc.End.Index != len(text)-1 {
		t.Fatalf("EOF span %+v %v", loc, err)
	}
	tree.SetStop(parser.GetCurrentToken())
	loc, err = parserLocation(file, tree)
	if err != nil || loc.End.Index != len(text) {
		t.Fatalf("explicit EOF: %+v %v", loc, err)
	}
	tree.SetStop(nil)
	loc, err = parserLocation(file, tree)
	if err != nil || loc.End != nil {
		t.Fatalf("unknown end %+v %v", loc, err)
	}
	tree.SetStart(nil)
	if _, err = parserLocation(file, tree); err == nil {
		t.Fatal("accepted missing start")
	}
}
func TestRuneLocationBounds(t *testing.T) {
	f, _ := NewSourceFile("s", "a😀")
	loc, err := f.RuneLocation(1, 2)
	if err != nil || loc.Start.Index != 1 || loc.End.Index != 3 {
		t.Fatalf("span %+v %v", loc, err)
	}
	loc, err = f.RuneLocation(2, 2)
	if err != nil || loc.Start != *loc.End {
		t.Fatal("empty EOF span")
	}
	for _, span := range [][2]int{{-1, 0}, {0, 3}, {2, 1}} {
		if _, err := f.RuneLocation(span[0], span[1]); err == nil {
			t.Fatalf("accepted %v", span)
		}
	}
}
